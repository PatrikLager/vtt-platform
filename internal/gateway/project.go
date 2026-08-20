package gateway

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

// Viewer is one seat, plus where it is looking from.
//
// A player's viewpoint is fixed — the union of the actors they control — and
// Viewpoint is IGNORED for them (visibility spec §3.1.1: "an unassigned PLAYER
// does not perch"). That is not tidiness. Viewpoint arrives from the client
// over SetViewpoint, so honouring it for a player would let any player name an
// NPC and look out of its eyes, which is session zero with an extra step.
//
// For a spectator it is the shoulder they are currently riding, and the perch
// must be a PLAYER-CONTROLLED actor: "a spectator perched on the Goblin Archer
// would watch the ambush from inside it, and the arc would be undone in a
// single click." Task 6 refuses such a perch at the command; eyes below
// refuses to honour one that arrived any other way.
type Viewer struct {
	ParticipantID string
	Role          identity.Role
	Viewpoint     string // spectator perch: an actor id, or ""
}

// Projector is one connection's projection of the log.
//
// WHAT THE FOUR MAPS ARE, because "per-connection state" is exactly the thing
// visibility spec §4.1 says this design does not have. They are not a cache of
// anything derivable from the current engine.State, and they are not an
// optimisation that could be deleted for a slower-but-equal recomputation.
// They are the RUNNING RESULT of the pure function §4.1 describes: the
// projection is a function of (log-so-far, viewer), "log-so-far" is a
// PREFIX, and a function of a growing prefix is a fold. These maps are that
// fold's accumulator. Feed two projectors the same prefix in the same order
// and they hold the same maps.
//
// TWO TESTS, because that sentence is two claims and one test only proves the
// weaker. TestTheSameLogProjectsTheSameStreamEveryTime runs the whole log
// through a fresh projector many times and pins DETERMINISM — same inputs,
// same envelopes, same order. What makes live streaming and catch-up agree is
// the stronger claim that the stream SPLITS: what a seat folded before it
// dropped, followed by a from-scratch projection of everything after its
// resume point, must land it exactly where it would have been had it never
// dropped. TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed performs that
// split at every point in a log and folds both halves.
//
// The purity is in (log-prefix, viewer) and NOT in (state, viewer) — see the
// consequence for Task 5 below, which is a fact about these maps and not a
// caution about them.
//
// They exist because the wire has no "here is your whole world" message. A
// viewer learns of a scene, an actor or a token exactly once, by an
// introduction, and both folds REFUSE a second one — engine.Apply returns
// "engine: scene %q already exists" / "actor %q already exists" / "token %q
// already exists", and client/src/fold.ts mirrors it in its own words
// (`duplicate scene` / `duplicate actor` / `duplicate token`), where a throw
// freezes that viewer's state (Task 3's finding). Permanently in the sense that
// matters: Session re-folds its whole APPEND-ONLY log on every event, so the
// poisoned envelope throws again on every one that follows. The only escape is
// a retraction covering that sequence — fold's pass 1 expands retracted ranges
// and pass 2 skips them — and nothing the viewer themselves can do reaches it.
// So the projection has to know what it has already introduced. TokenHidden is
// the one exception, tolerant on both sides by explicit ruling, which is why
// departures need no care here.
//
// A CONSEQUENCE FOR WHOEVER WIRES THIS UP (Task 5): a Projector must be fed
// the log from the beginning, discarding what precedes the client's resume
// point. It must NOT be seeded from a snapshot of engine.State, because
// `actors` is genuinely path-dependent and a snapshot cannot reconstruct it:
// an NPC seen at seq 7 and hidden at seq 8 is still in the viewer's ROSTER at
// seq 8 (TokenHidden removes a token, not an actor), while any function of
// state-at-8 alone says it is unknown. Such a projector would re-introduce
// that actor the moment it came back into view, and the duplicate ActorAdded
// is precisely the fold error above.
type Projector struct {
	viewer Viewer

	// scenes, actors: introduced to this viewer, and never withdrawn. There
	// is no un-introduce on the wire.
	scenes map[string]bool
	actors map[string]bool
	// tokens: on this viewer's board RIGHT NOW. Grows on arrival, shrinks on
	// departure — creatures are pure line of sight (spec §3.2).
	tokens map[string]bool
	// seen: the squares last reported visible, per scene, so an unchanged
	// view emits no SceneSeen. Compared, never merged: SceneSeen carries the
	// whole CURRENT set (spec §5) and the client is what accumulates.
	seen map[string]map[string]bool
}

func NewProjector(v Viewer) *Projector {
	return &Projector{
		viewer: v,
		scenes: map[string]bool{},
		actors: map[string]bool{},
		tokens: map[string]bool{},
		seen:   map[string]map[string]bool{},
	}
}

// Sight range and tolerance are INPUTS, not platform decisions (spec §3.4 and
// §3.3.1, CLAUDE.md rule 5). internal/sight reads both as "<= 0 means not
// supplied": unlimited range, and one exposed sample point out of nine.
//
// Passed as literals here rather than read off the actor, deliberately. An
// actor's sight range is a RULES fact, and reaching into Actor.Attributes for
// a well-known key would put game-system vocabulary in platform code — the
// rule 5 violation this arc has avoided everywhere else. When a ruleset
// supplies these they arrive as arguments through this seam; nothing else in
// this file changes.
const (
	sightRangeNotSupplied int32 = 0
	toleranceNotSupplied  int   = 0
)

// Project returns the envelopes THIS viewer should receive for one log event.
// Zero, one, or several: an event can be withheld, passed through, or turned
// into an introduction plus the event itself.
//
// st is the state AFTER env was folded, so "what can this viewer see now" is
// answered against the world the event just created.
//
// THE ENVELOPE IS SHARED. The pump hands the same *Envelope to every
// connection, so nothing here may write to env or to anything it points at —
// a redaction done in place would edit the DM's copy of history from inside a
// player's projection. Every envelope this function synthesizes is freshly
// built, and the only one it ever returns by pointer is env itself, unchanged.
// TestProjectingChangesNeitherTheEventNorTheState pins both halves.
func (pr *Projector) Project(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	if env == nil {
		return nil
	}
	switch pr.viewer.Role {
	case identity.RoleDM, identity.RoleAgent:
		// The identity projection, and it must stay pointer-identical: the
		// DM's and the agent's streams are byte-for-byte what they are today
		// (spec §3.1, exit criterion 8). Note it does not read st at all, so
		// no amount of projection machinery can slow or disturb them.
		return []*vttv1.Envelope{env}
	case identity.RolePlayer, identity.RoleSpectator:
		// projected below
	default:
		// An unknown role gets NOTHING. identity.Role is a string, so a
		// participant row written by a future version of this codebase — or
		// by hand — can carry a value this switch has never heard of, and
		// "some role we do not recognise" is the definition of uncertain
		// (spec §4.4).
		return nil
	}
	if st == nil {
		return nil // no world to look at; omit rather than guess
	}

	now := pr.look(st)
	v := pr.classify(env, now)
	if v == unrecognised {
		// FAIL CLOSED, and all the way: no event, and no derived consequences
		// either. The projection cannot tell what an unknown payload did to
		// the world, so it does not narrate the aftermath — it stays silent
		// and picks the diff up on the next event it does understand, whose
		// transitions are computed against state, not against history.
		return nil
	}
	out := pr.transitions(env.GetSequence(), now, st)
	if v == forwarded {
		out = append(out, redactedFor(env))
	}
	return out
}

// redactedFor is the envelope a projected viewer actually receives for a
// payload classify passed. For every payload but one it is env ITSELF, by
// pointer and unchanged — see Project's doc comment on why nothing here may
// write to a shared envelope.
//
// EventsRetracted is the exception, and the only payload this file both
// forwards and edits. Its range must reach every seat (classify says why) and
// its free-text reason must not, so the marker is CLONED and the text dropped;
// the clone is what makes this a redaction rather than the in-place edit that
// would rewrite the DM's copy. Envelope metadata — event id, actor role,
// participant, timestamp, session — travels as it does on every other
// forwarded envelope: it says who undid something, which is not a fact about
// the world.
func redactedFor(env *vttv1.Envelope) *vttv1.Envelope {
	if env.GetEventsRetracted().GetReason() == "" {
		return env // nothing to redact, so nothing to copy
	}
	out := proto.Clone(env).(*vttv1.Envelope)
	out.GetEventsRetracted().Reason = ""
	return out
}

// sightView is everything a single look at the world tells us about one
// viewer: which squares they can see (per scene), which tokens stand on those
// squares, and which actors they are entitled to know about.
type sightView struct {
	squares map[string]map[string]bool // scene id -> visible square keys
	tokens  map[string]bool            // token ids standing on a visible square
	actors  map[string]bool            // actor ids this viewer may know of
}

// look computes what this viewer can see of st. It READS state and never
// writes it (CLAUDE.md rule 4: engine.Apply is the only writer).
//
// RECOMPUTED ON EVERY EVENT, with no memo. Deciding which events "cannot
// change visibility" is exactly where a leak would hide — a wrong answer there
// is silent and permanent — and spec §4.4 says to omit when uncertain, not to
// guess cheaply. The cost is real and Task 1 measured its dominant term:
// sight.VisibleFrom is ~15ms on a sparse 60x60 and ~176ms on a dense one, per
// eye, per event. Spec §8 files this as a cliff to measure rather than a slope
// to worry about; if it becomes one, the memo key that would be SOUND is
// (scene id, eye position, the set of open doors), because terrain itself is
// immutable after SceneCreated and open doors are the only other input.
func (pr *Projector) look(st *engine.State) sightView {
	v := sightView{
		squares: map[string]map[string]bool{},
		tokens:  map[string]bool{},
		actors:  map[string]bool{},
	}
	if st == nil {
		return v
	}
	for _, eye := range pr.eyes(st) {
		// Unordered on purpose, and the only unordered walk in this file:
		// this loop UNIONS squares into a set, so Go's randomised map
		// iteration cannot change what comes out. The loops that emit
		// envelopes are a different matter — see the sorters at the bottom.
		for _, tok := range st.Tokens {
			if tok.ActorID != eye {
				continue
			}
			sc, ok := st.Scenes[tok.SceneID]
			if !ok {
				continue
			}
			// Created even when nothing is visible from it. Being IN a scene
			// is what earns a board, not being able to see anything from
			// where you stand (spec §4.2 — "you are not in a scene" and "you
			// are in a scene you cannot see" are different things).
			dst := v.squares[tok.SceneID]
			if dst == nil {
				dst = map[string]bool{}
				v.squares[tok.SceneID] = dst
			}
			for sq := range sight.VisibleFrom(sc, tok.X, tok.Y, sightRangeNotSupplied, toleranceNotSupplied) {
				dst[sq] = true
			}
		}
	}
	for id, tok := range st.Tokens {
		// Indexing a nil map is a legal read in Go, so a token in a scene
		// this viewer has no eye in simply reads false.
		if v.squares[tok.SceneID][squareKey(tok.X, tok.Y)] {
			v.tokens[id] = true
			v.actors[tok.ActorID] = true
		}
	}
	for id, a := range st.Actors {
		// Spec §5's one explicit exception: actors controlled by any player
		// are always known. "You know your party exists when the rogue is two
		// rooms away; you merely cannot see their token." Without this a
		// player's character list names every goblin in the dungeon with none
		// on screen — finding 14 one layer up.
		if len(a.GetControllerIds()) > 0 {
			v.actors[id] = true
		}
	}
	return v
}

// eyes are the actors this viewer sees through.
func (pr *Projector) eyes(st *engine.State) []string {
	switch pr.viewer.Role {
	case identity.RolePlayer:
		// The union of the actors they control (spec §3.1). Sight belongs to
		// actors; a participant inherits it from the actors they control.
		var ids []string
		for id, a := range st.Actors {
			for _, c := range a.GetControllerIds() {
				if c == pr.viewer.ParticipantID {
					ids = append(ids, id)
					break
				}
			}
		}
		sort.Strings(ids)
		return ids
	case identity.RoleSpectator:
		// One shoulder, and it must be a party member's. An unknown actor id
		// (including the empty one, meaning "not perched yet") and an NPC
		// both land here as no eyes at all.
		a, ok := st.Actors[pr.viewer.Viewpoint]
		if !ok || len(a.GetControllerIds()) == 0 {
			return nil
		}
		return []string{pr.viewer.Viewpoint}
	}
	return nil
}

// transitions are the envelopes that describe how this viewer's world CHANGED,
// synthesized from the difference between what they had been shown and what
// they can see now.
//
// Every one of them carries seq — the sequence of the event that CAUSED the
// change, not a number of its own (spec §4.2). Retraction is a range over
// sequence NUMBERS: campaign.retractedSet, harness.Fold and client/src/fold.ts
// all expand an EventsRetracted's inclusive [from,to] into a set and then SKIP
// any envelope whose sequence is in it, by number and never by identity. So
// retracting the event that revealed the goblin also drops the introduction it
// caused, and the player forgets the sighting.
//
// MEASURED, not reasoned, and the measurement found a limit worth writing down
// rather than a clean result. Folding one projected stream with and without a
// retraction of the revealing event: the player does forget the goblin — and
// if any LATER event about it was forwarded, their fold then fails on the
// dangling reference ("moved unknown token") where the DM's identical
// retraction folds cleanly, because the goblin's existence reached the DM at
// its own sequence and reached the player at the revealing one. campaign.Undo
// dry-runs the would-be fold before persisting, but it dry-runs the LOG, so a
// retraction safe for every seat that receives the log can still be unsafe for
// one that receives a projection of it. Not fixed here: the stamping is what
// spec §4.2 specifies and what this task's tests pin, and a per-viewer
// pre-flight is a gateway decision, not a projection one. Filed in the task
// report; spec §4.3's keystone is where it is catchable.
//
// Stamping also means several envelopes legitimately share one sequence.
// Nothing downstream requires them to be unique: the folds key on nothing but
// the number, and client/src/wire.ts advances its replay cursor with
// `if (env.sequence > this.lastSeq)`.
//
// THE ORDER IS LOAD-BEARING, because both folds are strict about what must
// already exist, in the same words: a scene before the tokens placed in it
// ("token placed in unknown scene" / "token placed on unknown scene"), an actor
// before its token ("token placed for unknown actor"), and the scene before its
// SceneSeen ("scene seen for unknown scene"). engine.Apply and
// client/src/fold.ts both, which is the mirror property Task 3 exists to keep.
//
// Departures before arrivals is the one pair the folds do NOT constrain — token
// ids are unique, so no arrival can collide with a departure whichever way
// round they go. Chosen anyway, because it is the order under which the viewer
// never holds more tokens than they are entitled to, not even for the length of
// one batch.
func (pr *Projector) transitions(seq int64, now sightView, st *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope

	for _, id := range sortedSceneIDs(now.squares) {
		if pr.scenes[id] {
			continue
		}
		sc := st.Scenes[id]
		// REDACTED: the outline, and nothing in it (spec §4.2). "Of course
		// there is a board, but you do not know what is in the black area
		// before you enter the black area." Knowing the room you stand in is
		// 7x3 is the shape of the paper, not a leak; its tiles and objects
		// arrive square by square through SceneSeen as the viewer looks
		// around. Legal on the wire since the tiles-optional ruling
		// (2026-08-13), so a redacted scene needs no new message.
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
				SceneId:    sc.ID,
				Name:       sc.Name,
				GridWidth:  sc.GridWidth,
				GridHeight: sc.GridHeight,
			}}})
		pr.scenes[id] = true
	}

	for _, id := range sortedSet(now.actors) {
		if pr.actors[id] {
			continue
		}
		a, ok := st.Actors[id]
		if !ok {
			continue
		}
		// Cloned, because st.Actors holds POINTERS into live engine state and
		// ActorControlGranted mutates an Actor in place. Handing the same
		// pointer out would put live state on a connection's wire, where a
		// later grant would retroactively change an envelope already sent.
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
				Actor: proto.Clone(a).(*vttv1.Actor)}}})
		pr.actors[id] = true

		// AND THE CONDITIONS ON IT, which do NOT ride along in the Actor.
		//
		// An actor's resources are fields of vttv1.Actor, so the clone above
		// already carries every ResourceChanged the viewer never saw. Its
		// conditions are not: engine.State keeps them in a separate map, so a
		// condition applied while this actor was out of sight would be missing
		// from the viewer's fold — and a later ConditionRemoved, which
		// classify forwards the moment the actor is known, is then a HARD
		// ERROR in both folds ("condition %q not present on actor %q" /
		// `condition "..." not present on actor "..."`). Task 3 measured what
		// that costs a client: Session re-folds its whole log on every event,
		// so one throw freezes that viewer's state permanently. Spec §8 names
		// this exact shape as the worst failure available, with TokenHidden as
		// its example; conditions are the same shape one layer over.
		//
		// The sequence stamped is the CAUSING one, like every other
		// synthesized envelope (spec §4.2), which means the viewer's
		// AppliedSeq for such a condition is the moment they learned of it
		// rather than the moment it was applied. That divergence is the same
		// family as §4.2's own ("the goblin should return to where it stood;
		// the player merely forgets it") and runs in the same direction: the
		// viewer's own history is late, never earlier than the truth.
		for _, c := range st.Conditions[id] {
			out = append(out, &vttv1.Envelope{Sequence: seq,
				Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
					ActorId: id, ConditionId: c.ID, Source: c.Source}}})
		}
	}

	for _, id := range sortedSet(pr.tokens) {
		if now.tokens[id] {
			continue
		}
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: id}}})
		delete(pr.tokens, id)
	}

	for _, id := range sortedSet(now.tokens) {
		if pr.tokens[id] {
			continue
		}
		tok := st.Tokens[id]
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
				TokenId: id, SceneId: tok.SceneID, ActorId: tok.ActorID,
				Position: &vttv1.GridPosition{X: tok.X, Y: tok.Y}}}})
		pr.tokens[id] = true
	}

	for _, id := range sortedSceneIDs(now.squares) {
		if sameSet(pr.seen[id], now.squares[id]) {
			continue
		}
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_SceneSeen{SceneSeen: sceneSeenFor(st.Scenes[id], now.squares[id])}})
		// Stored, not merged. now.squares is rebuilt from scratch by every
		// look, so this never aliases anything a later call mutates.
		pr.seen[id] = now.squares[id]
	}

	return out
}

// sceneSeenFor is the whole of what this viewer can see of sc right now
// (spec §5: the whole current visible set, never a delta). Idempotent by
// construction, which is what lets the projection keep no record of which
// squares it has already sent.
func sceneSeenFor(sc engine.Scene, squares map[string]bool) *vttv1.SceneSeen {
	ss := &vttv1.SceneSeen{SceneId: sc.ID, Tiles: map[string]*vttv1.TileRef{}}
	for sq := range squares {
		t, ok := sc.Tiles[sq]
		if !ok {
			// A visible square with no terrain recorded. Legal — a scene may
			// declare no tiles at all — and it contributes nothing to send.
			continue
		}
		ss.Tiles[sq] = &vttv1.TileRef{Kind: t.Kind, Material: t.Material, Art: t.Art}
	}
	for _, o := range sc.Objects {
		if !objectInSight(o, sc, squares) {
			continue
		}
		ss.Objects = append(ss.Objects, &vttv1.SceneObject{
			ObjectId: o.ObjectID, Kind: o.Kind,
			At:    &vttv1.GridPosition{X: o.X, Y: o.Y},
			Width: o.Width, Height: o.Height,
			RotationDegrees: o.RotationDegrees,
			BlocksSight:     o.BlocksSight, BlocksMove: o.BlocksMove,
			Art: o.Art,
		})
	}
	return ss
}

// objectInSight reports whether any square of o's footprint is visible.
//
// An object covering NO square — Width or Height below 1 — is never revealed,
// which is the same rule sight.Blockers applies when deciding what casts a
// shadow: movement's covers() admits no square once the extent is zero or
// negative, so sight agrees and so does this. A degenerate footprint therefore
// blocks nothing and is shown to nobody, rather than blocking sight while
// staying invisible.
//
// The walk is CLAMPED TO THE GRID and its far edge compared in int64, for two
// independent reasons. A merely enormous footprint would otherwise spin over
// billions of squares that VisibleFrom never even looked at, because it only
// ever walks the declared grid; and one whose extent overflows int32 —
// sight.go carries the same unreachable-but-undefended case — would wrap and
// be walked backwards or not at all. Squares outside the grid cannot be in
// `squares`, so the clamp costs no correctness.
func objectInSight(o engine.SceneObject, sc engine.Scene, squares map[string]bool) bool {
	if o.Width < 1 || o.Height < 1 {
		return false
	}
	for y := max(o.Y, 0); y < sc.GridHeight && int64(y) < int64(o.Y)+int64(o.Height); y++ {
		for x := max(o.X, 0); x < sc.GridWidth && int64(x) < int64(o.X)+int64(o.Width); x++ {
			if squares[squareKey(x, y)] {
				return true
			}
		}
	}
	return false
}

// verdict is what the projection decided about ONE payload.
type verdict int

const (
	// unrecognised: the projection has never heard of this payload. It emits
	// NOTHING — see Project.
	unrecognised verdict = iota
	// withheld: understood, and not this viewer's to have. Either it leaks,
	// or the projection introduces its subject itself.
	withheld
	// forwarded: understood, and safe for this viewer exactly as written.
	forwarded
)

func passIf(ok bool) verdict {
	if ok {
		return forwarded
	}
	return withheld
}

// classify decides what happens to one payload, and it is the security
// boundary of this whole arc.
//
// EXHAUSTIVE OVER THE ENVELOPE ONEOF, with default returning unrecognised
// (spec §4.4). A default that forwards is how this ships broken: AttackRolled
// names a target, NarrationAdded may describe a room, ConditionApplied names
// an actor, and a note can say anything.
// TestEveryEnvelopePayloadArmHasAnExplicitRuling walks the descriptor and
// reds if a new arm ever lands without a ruling here.
//
// Called BEFORE transitions, so pr.tokens still holds what the viewer had
// BEFORE this event — which is exactly the question TokenMoved needs answered.
func (pr *Projector) classify(env *vttv1.Envelope, now sightView) verdict {
	// knows is "will this viewer hold that actor by the time this envelope
	// folds". pr.actors is what they already have; now.actors is what
	// transitions is about to introduce AHEAD of the forwarded envelope. An
	// empty id names no actor and constrains nothing.
	knows := func(ids ...string) bool {
		for _, id := range ids {
			if id != "" && !pr.actors[id] && !now.actors[id] {
				return false
			}
		}
		return true
	}

	switch p := env.GetPayload().(type) {
	// --- forwarded: table-level, and about no scene, actor or square -------

	case *vttv1.Envelope_SessionStarted, *vttv1.Envelope_SessionEnded:
		// Everyone at the table is at the same table. A session's name and
		// its start and end are the sitting itself, not anything in the
		// world, and the client's session panel is built from them.
		return forwarded

	case *vttv1.Envelope_EventsRetracted:
		// The one payload whose WITHHOLDING is the dangerous direction. A
		// retraction erases history; a viewer who never receives it keeps
		// folding an event the table has agreed did not happen. Skipping
		// sequence numbers a viewer never received is a no-op on their side,
		// so the RANGE is free to forward.
		//
		// THE REASON IS NOT. An earlier draft of this comment justified the
		// arm with "it carries only sequence numbers", which is false:
		// EventsRetracted.reason (contract field 3) is free text handed
		// straight through campaign.Undo from whoever asked for the undo, and
		// "undo, I put the archer on the wrong square" is the NoteUpserted
		// ruling's own argument under a different message name. Forwarded with
		// the reason redacted — see redactedFor — which keeps the dangerous
		// direction safe without leaving a free-text channel open.
		// FLAGGED FOR ADJUDICATION: costs a player nothing they can see today.
		// NOTHING IN THE CLIENT EVER READS THE REASON — client/src/view/feed.ts
		// calls the marker bookkeeping and DROPS it ("rendering it too would
		// narrate the erasure"), and fold.ts and undo.ts read only its range.
		//
		// The MARKER, unlike its reason, is not invisible, and an earlier
		// version of this comment said it was. client/src/view/spectator.ts's
		// Events ticker walks the RAW log and falls through to describe()'s
		// default arm (`return p.case ?? "event"`), so a retraction surfaces
		// there as a bare "#N eventsRetracted" row. That is consistent with
		// forwarding — the row names no world fact, and the seat is entitled to
		// know history was erased — but it is why this ruling is about the
		// reason field and not about the marker. Recorded because of HOW the
		// wrong version was reached: it was checked by grepping every site that
		// NAMES eventsRetracted, and the ticker renders it precisely by not
		// naming it.
		return forwarded

	case *vttv1.Envelope_NarrationAdded:
		// RULED, not obvious, and the opposite of the note ruling below.
		// Narration is ADDRESSED: someone at the table wrote it to be heard,
		// and add_narration is open to dm/agent/player alike
		// (commandRoles in authz.go). The feed IS the log for narration
		// (world-layer spec §4), so withholding it does not redact a detail,
		// it silences the table's story channel and leaves players with a
		// board and no game. What a DM chooses to say out loud is the DM's
		// editorial call, the same way it is at a physical table.
		// FLAGGED FOR ADJUDICATION: spec §4.4 names NarrationAdded as a
		// reason the default must not forward, but rules on no arm.
		return forwarded

	// --- withheld: the projection introduces these itself ------------------

	case *vttv1.Envelope_SceneCreated:
		// A scene reaches a viewer only when they are standing in it, and
		// then REDACTED (spec §4.2, exit criterion 6). Six loaded scenes must
		// not hand a player six names and sizes — a table of contents for an
		// adventure they have not played. transitions synthesizes the one
		// they are in; this arm is why they get no others.
		return withheld

	case *vttv1.Envelope_ActorAdded:
		// Introduced by transitions when the actor becomes knowable — first
		// sight for an NPC, immediately for anything a player controls
		// (spec §5). Withheld here rather than conditionally forwarded so
		// that there is exactly ONE code path that introduces an actor:
		// two would eventually both fire on the same event, and a duplicate
		// ActorAdded is a fold error, which on the client is a permanent
		// state freeze.
		return withheld

	case *vttv1.Envelope_TokenPlaced:
		// Same single-path reasoning as ActorAdded, plus the obvious one:
		// this is the payload session zero leaked. A placement reaches a
		// viewer only as an arrival synthesized from what they can see.
		return withheld

	case *vttv1.Envelope_TokenHidden, *vttv1.Envelope_SceneSeen:
		// PROJECTION-ONLY (spec §5): no command produces either, so neither
		// can structurally reach the log, and one arriving from the log is a
		// fact about the world nobody can explain. Withheld rather than
		// trusted — the projection issues these, it does not relay them.
		return withheld

	case *vttv1.Envelope_NoteUpserted, *vttv1.Envelope_NoteDeleted:
		// RULED, and the counterpart to the narration ruling above. A note is
		// a durable world RECORD, not an utterance: upsert_note/delete_note
		// are dm/agent only ("world facts are the DM's", world-layer spec
		// §5), nothing addresses it to anyone, and "Archer waits at 19,8" is
		// a perfectly ordinary note. Spec §4.4's "a note can say anything" is
		// the argument, and with nothing in the payload to project against,
		// uncertain means omit.
		// FLAGGED FOR ADJUDICATION: today's client shows the notes panel to
		// players and spectators, so this ruling empties it for them.
		return withheld

	case *vttv1.Envelope_AdventureLoaded:
		// Testimony that names an adventure by id and title — the shape of
		// the §4.2 leak with the campaign rather than a goblin as its
		// subject. It is an engine no-op, and everything it means arrives via
		// the events in its own compile batch, each projected on its own
		// merits, so withholding costs a viewer nothing they are entitled to.
		return withheld

	// --- forwarded only when the viewer can already see the subject --------

	case *vttv1.Envelope_TokenMoved:
		// Requires visible BEFORE and AFTER, and the "before" is the half
		// that matters: TokenMoved names BOTH ends of the walk, so forwarding
		// a move whose `from` the viewer never saw hands them the hidden
		// square this arc exists to protect. A move that ends out of sight is
		// withheld too, and transitions has already emitted the TokenHidden
		// that explains it.
		id := p.TokenMoved.GetTokenId()
		return passIf(pr.tokens[id] && now.tokens[id])

	case *vttv1.Envelope_DoorOpened:
		return passIf(pr.canSeeSquare(now, p.DoorOpened.GetSceneId(), p.DoorOpened.GetAt()))

	case *vttv1.Envelope_DoorClosed:
		// A door you cannot see does not visibly move. Both arms are safe on
		// the fold's terms too: a visible square implies an eye in that
		// scene, which implies the scene was introduced, so neither can
		// arrive for a scene the viewer does not have.
		return passIf(pr.canSeeSquare(now, p.DoorClosed.GetSceneId(), p.DoorClosed.GetAt()))

	// --- forwarded only when the viewer knows every actor named ------------
	//
	// Knowing an actor is a weaker test than seeing its token, and
	// deliberately so: a party member two rooms away is known and unseen
	// (spec §3.2/§5), and hearing that the rogue took damage is not the same
	// as being shown where they are standing. None of these payloads carries
	// a position.

	case *vttv1.Envelope_AttackRolled:
		return passIf(knows(p.AttackRolled.GetAttackerId(), p.AttackRolled.GetTargetId()))

	case *vttv1.Envelope_AbilityUsed:
		return passIf(knows(append([]string{p.AbilityUsed.GetActorId()},
			p.AbilityUsed.GetTargetIds()...)...))

	case *vttv1.Envelope_ActorControlGranted:
		return passIf(knows(p.ActorControlGranted.GetActorId()))

	case *vttv1.Envelope_ActorControlRevoked:
		// A revoke can be the event that makes an actor an NPC. The viewer
		// already holds it either way — pr.actors never forgets — so the
		// grant/revoke pair stays coherent on their side.
		return passIf(knows(p.ActorControlRevoked.GetActorId()))

	// --- forwarded only to a viewer who ALREADY HELD the actor -------------
	//
	// A STRICTER TEST than knows(), and the three arms below are the only ones
	// that need it. knows() is satisfied by an actor transitions is about to
	// introduce, which is right for testimony — an attack on a goblin that
	// just walked into view is news. It is wrong for anything whose effect the
	// INTRODUCTION ITSELF already carries, because the viewer would then apply
	// it twice:
	//
	//   - resources are fields of vttv1.Actor, so the introduction's clone is
	//     already post-change, and both folds RECOMPUTE from the value the
	//     viewer currently holds and reject a mismatch ("event new_value N does
	//     not match computed M"). Note that this catches a second application
	//     only where the recomputation DISAGREES, and there are three shapes
	//     where it does not: a delta of 0, at any value; already 0 with a
	//     negative delta; and already at Max with a positive one, this last
	//     only when Max > 0, since both folds read a non-positive Max as
	//     unlimited and never clamp against it. In those the duplicate folds
	//     silently into a wrong board instead of raising a loud error.
	//     Withholding is what makes which of the two happens moot;
	//   - conditions travel with the introduction (see transitions), and both
	//     folds reject a duplicate outright.
	//
	// ActorControlGranted/Revoked have the same shape and are NOT here,
	// deliberately: their effect is also in the clone, but a second application
	// is a no-op in both folds rather than a freeze. That is the whole
	// distinction — not whether the fact is duplicated, but whether the fold
	// survives the duplicate. The two arms reach it differently, and only the
	// first is idempotent by an explicit check: GRANTED tests membership and
	// returns early, in both folds and under that word (engine.Apply's
	// "idempotent: already controls it" / fold.ts's `if
	// (a.controllerIds.includes(v.participantId)) return;`), while REVOKED is
	// idempotent BY CONSTRUCTION — each fold rebuilds the controller list as a
	// filter that drops the id, and filtering an id that is already absent
	// changes nothing. No guard, and none needed.
	//
	// Nothing is lost by withholding: the state these events carry reached the
	// viewer in the introduction, on the same event, one envelope earlier.

	case *vttv1.Envelope_ResourceChanged:
		return passIf(pr.actors[p.ResourceChanged.GetActorId()])

	case *vttv1.Envelope_ConditionApplied:
		return passIf(pr.actors[p.ConditionApplied.GetActorId()])

	case *vttv1.Envelope_ConditionRemoved:
		return passIf(pr.actors[p.ConditionRemoved.GetActorId()])

	default:
		return unrecognised
	}
}

func (pr *Projector) canSeeSquare(now sightView, sceneID string, at *vttv1.GridPosition) bool {
	if at == nil {
		return false
	}
	return now.squares[sceneID][squareKey(at.GetX(), at.GetY())]
}

// squareKey formats a square the way the wire does: column then row, comma
// separated (maps-as-geometry spec §4.1).
//
// A fourth copy of a three-line format — engine's gridKey, mapdef's and
// sight's squareKey are the others — and unlike a duplicated constant this one
// cannot drift silently: every visibility answer in this file is a lookup of a
// key built here inside a map built by sight.VisibleFrom. Disagree about the
// format and NOTHING is ever visible to anyone, which the suite reports
// immediately and loudly.
func squareKey(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// The two sorters below exist for one reason: Go randomises map iteration,
// and this file turns sets into an ORDERED stream of envelopes. Unsorted, two
// runs of the same log would emit the same envelopes in different orders — a
// coin flip under the byte-for-byte parity the keystone (spec §4.3) rests on,
// and a nightmare to reproduce. TestTheSameLogProjectsTheSameStreamEveryTime is
// what holds them here.

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedSceneIDs(m map[string]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
