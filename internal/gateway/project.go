package gateway

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

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
// must be a PARTY MEMBER: "a spectator perched on the Goblin Archer would watch
// the ambush from inside it, and the arc would be undone in a single click."
// MayPerch refuses such a perch at the command (viewpoint.go, wired into
// Authorize); eyes below refuses to honour one that arrived any other way. Both
// ask isPartyMember, which reads the ACTOR'S KIND and not who holds it — the
// sentence here used to say "player-controlled", and that gap is what spec §5.1
// closed.
type Viewer struct {
	ParticipantID string
	Role          identity.Role
	Viewpoint     string // spectator perch: an actor id, or ""
}

// Projector is one connection's projection of the log.
//
// WHAT THE FIVE MAPS ARE, because "per-connection state" is exactly the thing
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
// freezes that viewer's state (Task 3's finding). Permanently, and the word
// got STRONGER on 2026-08-31: Session re-folds its whole APPEND-ONLY log on
// every event, so the poisoned envelope throws again on every one that
// follows, and there is now no escape at all. Until this arc there was one —
// a retraction covering that sequence, which the fold skipped — and retraction
// has left the platform, so a duplicate introduction is a viewer who never
// recovers. So the projection has to know what it has already introduced.
// TokenHidden is the one exception, tolerant on both sides by explicit ruling,
// which is why departures need no care here.
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
	// doors: the door squares this viewer believes OPEN, per scene. Unlike
	// tokens this is a belief about TERRAIN, so it is only ever corrected for
	// squares the viewer can currently see — a door that falls out of sight
	// keeps whatever the viewer last saw, exactly as explored terrain does.
	doors map[string]map[string]bool
}

func NewProjector(v Viewer) *Projector {
	return &Projector{
		viewer: v,
		scenes: map[string]bool{},
		actors: map[string]bool{},
		tokens: map[string]bool{},
		seen:   map[string]map[string]bool{},
		doors:  map[string]map[string]bool{},
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
	out := pr.transitions(env, env.GetSequence(), now, st)
	if v == forwarded {
		out = append(out, env)
	}
	return out
}

// perchSequence is the sequence every frame a perch produces carries: ZERO,
// which is not a sequence at all, and that is the point.
//
// A PERCH HAS NO CAUSING EVENT, so it has no number to inherit. Spec §4.2's
// rule — a synthesized envelope carries the sequence of the event that caused
// it — is right for every OTHER synthesized frame in this file and wrong here,
// and the difference is not cosmetic: a number that names nothing is a number
// something else can name. It was measured while undo still existed. A perch
// stamped with the seat's last folded sequence fell inside the range of an undo
// of that sequence — an event the watcher had never even received — which
// emptied their board with no message and left the party's next move dangling
// against a token that was no longer there ("moved unknown token").
//
// THE UNDO THAT FOUND IT IS GONE (2026-08-31: the log only goes forward), so
// that particular collision cannot recur. The rule it demonstrated is not
// about undo: a live number invites any future feature that addresses ranges
// of the log to address a frame no event caused. Zero is outside every such
// range by construction, on both sides of the wire.
//
// It also cannot move a replay cursor: client/src/wire.ts advances only on
// `env.sequence > lastSeq`, and seat.catchUp takes its head from projected
// output that a perch is never part of (the read loop that accepts a perch does
// not run until catch-up has returned).
//
// The cost, stated so nobody has to rediscover it: a perch frame is not
// addressable. A client cannot resume "just after" one — on a reconnect the
// perch is gone anyway (spec §3.1.1) and the client re-sends it, which is the
// same answer.
const perchSequence int64 = 0

// reperch moves a spectator onto a new shoulder and returns the envelopes that
// bring their board up to what those eyes can see (spec §3.1.1: "you can choose
// to shift to another character's view, whenever").
//
// IT IS transitions WITH NO CAUSING EVENT, and that is the whole implementation
// — which is the point. Nothing about hopping is a special case: the new view is
// the same diff against the same memory that every event already produces, so a
// perch introduces what the new eyes can see, hides the creatures the old
// shoulder could see and this one cannot, and leaves TERRAIN alone. That last
// one is not a decision taken here; it falls out of there being no message that
// un-explores a square, which is why "the bird remembers every shoulder it has
// sat on" is a property of the design rather than a feature of this function.
//
// SAT ON MEANS SAT ON, and the sentence needs that qualification now: it is
// every shoulder this function was CALLED with, which is not every shoulder the
// spectator named. perchBox coalesces a burst of hops to the one it ended on, so
// a shoulder superseded before the pump takes it is never applied here and its
// rooms never reach the board — measured, and written up at perchBox with what
// the difference is. The bird still remembers everywhere it alighted; it is only
// the places it flew over that leave nothing behind. And they are recoverable at
// any time: hop back and this function serves that shoulder in full, because
// what it computes is a diff against MEMORY, and the memory never held it.
// TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt pins that, because
// coalescing is indefensible without it.
//
// NO DOOR IS SKIPPED, unlike the event path: doorTransitions skips the square a
// door event is ABOUT because classify forwards that event itself, and here
// there is no event to forward. Every visible door this viewer believes wrong
// is corrected, which is exactly what a new pair of eyes needs.
func (pr *Projector) reperch(actorID string, st *engine.State) []*vttv1.Envelope {
	pr.viewer.Viewpoint = actorID
	if st == nil {
		// Omit rather than guess (spec §4.4), the same answer Project
		// gives to the same input. MEASURED, not assumed: deleting this
		// line ALONE changes nothing a test can see, because look guards
		// nil itself and transitions reads st only inside loops that a
		// viewer with no visible scene never enters. It stays because
		// that is a fact about two other functions rather than about
		// this one — delete look's guard too and the pair panics, which
		// is what TestASeatPerchesOnlyAgainstAWorldItHasSeen catches.
		return nil
	}
	return pr.transitions(nil, perchSequence, pr.look(st), st)
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
		// Spec §5's one explicit exception: PARTY MEMBERS are always known.
		// "You know your party exists when the rogue is two rooms away; you
		// merely cannot see their token." Without this a player's character
		// list names every goblin in the dungeon with none on screen — finding
		// 14 one layer up.
		//
		// isPartyMember (viewpoint.go) rather than a predicate spelled out
		// here, and rather than the "has any controller" this used to read. §5
		// says controlled by any PLAYER; the code said has any CONTROLLER, and
		// one grant_actor_control on a hidden monster published its whole
		// cloned Actor to every player at the table. Spec §5.1 has the ruling;
		// its migration rule was deleted 2026-08-24 and an absent kind is now
		// simply not a party member.
		if isPartyMember(a) {
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
		// One shoulder, and it must be a party member's — isPartyMember
		// (viewpoint.go), the same predicate MayPerch refuses on, so this
		// second refusal cannot drift from the first (spec §8, defence in
		// depth). An unknown actor id (including the empty one, meaning "not
		// perched yet") and anything that is not a party member both land here
		// as no eyes at all.
		a, ok := st.Actors[pr.viewer.Viewpoint]
		if !ok || !isPartyMember(a) {
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
// change, not a number of its own (spec §4.2). A SHARED NUMBER IS A DELIBERATE
// CONSEQUENCE, and it used to have a second one: while undo existed, a range
// over sequence numbers dropped the introductions a retracted event had caused
// along with the event itself, and a projected seat could be left folding a
// dangling reference where the DM's identical undo folded cleanly — because
// the goblin's existence reached the DM at its own sequence and the player at
// the revealing one. That asymmetry left with undo (2026-08-31: the log only
// goes forward): no command produces a ranged marker, no role may ask for one,
// and campaign.Undo — the last thing that could append one — is gone too, so
// stamping a derived envelope with its cause's number costs a seat nothing.
//
// Stamping also means several envelopes legitimately share one sequence, and
// before this task every envelope had a unique one. An introduction batch can
// be five. THE FOLDS TOLERATE THAT; THE RESUME CURSOR DOES NOT, and an earlier
// version of this comment read the first half as a clearance for both.
//
// True: both folds key on nothing but the number, so a shared sequence is
// nothing to them. Also true: client/src/wire.ts:175 advances its replay
// cursor with `if (env.sequence > this.lastSeq)`. That second fact is not the
// reassurance it looks like — it means lastSeq reaches N on the FIRST envelope
// of a five-envelope batch, while the other four are still in flight, and
// server.go resumes STRICTLY after a sequence (parseAfter at :363, handed to
// SubscribeWithNoProgressTimeout at :411). So a connection dropped after 2 of 5
// has NO EXPRESSIBLE RESUME POINT: `after=N` discards the three it never got,
// and `after=N-1` re-sends the two it already folded.
//
// UNRESOLVED, AND IT BELONGS TO WHOEVER WIRES THE PUMP (Task 5), because
// neither recovery is safe and the choice is not the projection's to make.
// Losing the tail drops a TokenHidden, which leaves an enemy token on a
// player's board — the leak direction this arc exists to close. Re-sending the
// head double-folds it, and client/src/session.ts:158-160 pushes every
// envelope onto its log with no dedupe, so a repeated ActorAdded is the
// duplicate-introduction freeze described above. What is needed is a resume
// point that can name a POSITION WITHIN a sequence, or a batch that cannot be
// torn; this file cannot supply either.
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
// CAUSE MAY BE NIL, and that is how a perch says "no event caused this" (see
// reperch). Everything below reads the ENVELOPE for exactly one thing — which
// door square, if any, this event is already about — and doorSubject answers
// nil the same way it answers any other payload that is not a door.
func (pr *Projector) transitions(cause *vttv1.Envelope, seq int64, now sightView, st *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope

	// pr.scenes AND pr.seen CANNOT OUTLIVE THE WORLD, and until 2026-08-31 they
	// could. A loop stood here that forgot any scene missing from st, because an
	// undo covering a SceneCreated removed that scene from the state seat.receive
	// re-folds while these maps still held it. st.Scenes is a VALUE map, so
	// anything reported about a scene it had lost named the ZERO Scene and put an
	// empty scene id on the wire — measured then: the returning scene produced a
	// TokenPlaced and a SceneSeen and BOTH were rejected ("token placed in
	// unknown scene", "scene seen for unknown scene"), which spec §8 calls the
	// worst failure available because client/src/session.ts re-folds its whole log
	// on every event and the throw therefore recurs forever.
	//
	// THE LOOP LEFT WITH RETRACTION, on a premise checked rather than assumed:
	// nothing removes a scene from engine.State. engine.Apply's SceneCreated arm
	// only inserts, DoorOpened/DoorClosed/SceneSeen only update in place, and no
	// code anywhere deletes from st.Scenes. seat.received only grows, so the scene
	// set of the fold it drives is monotone and pr.scenes is a subset of st.Scenes
	// at every call. What was left was not a cheap guard but a SILENT one: the
	// only thing that could now trigger it is a caller feeding one Projector
	// states from two different logs, and quietly forgetting is the wrong answer
	// to that — unreachable defence in this codebase fails LOUD (campaign's poison
	// contract, receive's FAIL CLOSED).
	//
	// SO WHY NOTHING RATHER THAN A LOUD GUARD, since that argument reaches only as
	// far as "not this loop": because the gate decides it. This package is
	// mutation-gated, so a guard here is a branch whose mutants must be killed by
	// a test, and the only tests that could reach one are tests that build an
	// impossible world by hand — three of them stood in project_test.go and went
	// with the loop, which records which and what they said. A loud guard would
	// buy the same unreachable branch back under a better name, and the reader
	// after it would face this decision again with one more adjudication attached.
	// The bare st.Scenes[id] in the SceneSeen walk below states the invariant
	// instead, and says where it comes from.
	//
	// pr.actors WAS NEVER GUARDED BY IT and still is not, which is worth carrying
	// forward rather than losing with the loop: actors are not scene-scoped and
	// pr.actors never forgets by design (see the Projector doc comment). Nothing
	// removes an actor from the world today either. The day something does, both
	// maps need an answer chosen deliberately, and this note is where that reader
	// should start.

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

	// AFTER the scene introductions above and never before them: both folds
	// reject a door in a scene they do not have ("door opened in unknown
	// scene").
	out = append(out, pr.doorTransitions(cause, seq, now, st)...)

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
		clone := proto.Clone(a).(*vttv1.Actor)

		// AN INTRODUCTION CANNOT CARRY A CONTROLLER, so who holds this actor
		// travels as GRANTS behind it — one per controller, in the set's own
		// order (visibility spec §5.1, 2026-08-24).
		//
		// This is not cosmetic and it is not a redaction: both folds now REFUSE
		// an ActorAdded that names a controller ("creating an actor does not
		// hand it to anyone"), so an introduction built from a clone of live
		// state was unfoldable the moment anyone had been granted anything.
		// Measured, not reasoned: TestAProjectedStreamFoldsCleanly and
		// TestAConditionAppliedOutOfSightArrivesWithTheActor both failed on
		// exactly that error before this loop existed.
		//
		// It also makes the projection say the same thing the log says. A
		// viewer who joins late gets creation-then-grant just as the log has
		// it, rather than a shape no log could hold; and a viewer already
		// present gets the grant twice — once here, once forwarded — which both
		// folds treat as idempotent by an explicit membership check.
		//
		// The KIND rides on each grant as well as on the clone. The fold's
		// grant arm writes kind only when the grant states one, so an
		// UNSPECIFIED here would leave the clone's kind standing — correct, but
		// only by accident of ordering. Stating it makes each envelope true on
		// its own.
		controllers := clone.GetControllerIds()
		clone.ControllerId = ""
		clone.ControllerIds = nil
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: clone}}})
		for _, cid := range controllers {
			out = append(out, &vttv1.Envelope{Sequence: seq,
				Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
					ActorId: id, ParticipantId: cid, Kind: a.GetKind()}}})
		}
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

	// THE UNION of what this viewer was last told and what they can see now,
	// never merely the latter. A scene the viewer can no longer see ANYTHING of
	// has no entry in now.squares at all — look() creates one per scene an eye's
	// token stands in, so a seat with no eye left in a scene has no key for it —
	// and a walk over now.squares alone would emit nothing, leaving pr.seen at
	// whatever was last visible. That is a leak on the client, which reads the
	// newest SceneSeen as its CURRENT visible set: the room stays lit forever.
	// Walking the union keeps the scene in play for exactly one more step, and
	// sameSet reports the difference between the last non-empty set and nothing.
	for _, id := range sortedSceneIDsUnion(pr.seen, now.squares) {
		// st.Scenes[id] IS PRESENT for every id this walk can produce, and since
		// 2026-08-31 the world itself guarantees it rather than a forgetting
		// loop above: now.squares only holds scenes look() found in st, pr.seen
		// only ever gains a key for a scene that was in st when it was reported,
		// and nothing removes a scene from st. A plain lookup rather than a
		// guarded one, because a guard here would be a branch no test could
		// reach.
		sc := st.Scenes[id]
		lit, inSight := now.squares[id]
		if sameSet(pr.seen[id], lit) {
			continue
		}
		// An EMPTY SceneSeen when the scene has gone dark, which needs no new
		// message and no new field: sceneSeenFor's contract is "the whole of
		// what this viewer can see of sc right now", and right now that is
		// nothing. It unions nothing into the client's Explored either, so
		// remembered terrain survives being darkened — pinned by
		// TestAnEmptySceneSeenDarkensTheSceneAndForgetsNoTerrain in
		// internal/engine and by "an empty sceneSeen darkens the scene and
		// forgets no terrain" in client/test/visibility.test.ts, both of which
		// send an EMPTY one and then assert Explored is intact.
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_SceneSeen{SceneSeen: sceneSeenFor(sc, lit)}})
		if !inSight {
			// FORGOTTEN, so the union no longer contains this scene and the
			// empty envelope cannot become a per-event heartbeat. Re-entering
			// the scene later starts from nil again, which is what an absent
			// key already meant.
			delete(pr.seen, id)
			continue
		}
		// Stored, not merged. now.squares is rebuilt from scratch by every
		// look, so this never aliases anything a later call mutates.
		pr.seen[id] = lit
	}

	return out
}

// sceneSeenFor is the whole of what this viewer can see of sc right now
// (spec §5: the whole current visible set, never a delta). Idempotent by
// construction, which is what lets the projection keep no record of which
// squares it has already sent.
func sceneSeenFor(sc engine.Scene, squares map[string]bool) *vttv1.SceneSeen {
	// THE SQUARE SET ITSELF, carried as itself rather than inferred downstream
	// from the terrain below. Everything after this line projects `squares`
	// through Tiles and loses whichever squares declare none — which is every
	// square of a bare canvas, and the server has already decided those are
	// visible and already sent the tokens standing on them. Field 4 is what
	// stops the client re-deriving a different answer from a lossy proxy
	// (visibility spec §5, Patrik's ruling 2026-08-22).
	//
	// SORTED, because `visible` is the only ordered field this function builds
	// FROM A MAP. `objects` is repeated too, but it walks the sc.Objects slice
	// and inherits its order. The tiles MAP is stable for a different reason
	// again, and it is worth naming precisely: not because a JSON object has no
	// order — it plainly does — but because protojson SORTS map entries before
	// writing them (encoding/protojson's marshalMap calls order.RangeEntries
	// with GenericKeyOrder). That is a guarantee of the ENCODER, so it would
	// stop holding if the encoder changed; this field's is a guarantee of this
	// line. An unsorted walk here would make two runs of one log emit different
	// bytes.
	// TestTheVisibleSetIsSentInAStableOrder pins it directly rather than
	// leaving it to the determinism property to catch by coin flip.
	ss := &vttv1.SceneSeen{
		SceneId: sc.ID,
		Tiles:   map[string]*vttv1.TileRef{},
		Visible: sortedSet(squares),
	}
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
		// sight for anything that is not a party member, immediately for one
		// that is (spec §5, keyed by isPartyMember since §5.1; this sentence
		// said "anything a player controls" while the rule still did, and both
		// halves of that were wrong in the same direction — a monster a player
		// holds is NOT introduced immediately, and a party member the DM holds
		// IS). Withheld here rather than conditionally forwarded so
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

	case *vttv1.Envelope_TokenRemoved:
		// retraction-leaves Task 8, and the SAME single-path reasoning as
		// TokenPlaced directly above, run in the other direction. A token's
		// departure from a viewer's board — whichever of three reasons
		// causes it: it walked out of sight, its actor lost every eye that
		// could see it, or (this arm) it was REMOVED from the world — is
		// always told the same way: transitions' own "for _, id := range
		// sortedSet(pr.tokens)" loop, which already emits TokenHidden the
		// moment `now.tokens` stops containing an id `pr.tokens` still holds
		// (this event's fold deletes the token from engine.State, so `now`
		// no longer contains it). Forwarding the raw TokenRemoved here as
		// well would be a SECOND path narrating the same disappearance, and
		// for a viewer who never held the token in the first place,
		// forwarding it would leak that a token existed and was removed
		// off-screen — exactly the kind of leak withheld exists to prevent.
		// A viewer who DID see it gets the TokenHidden that same loop
		// already produces; nothing here is lost. Pinned end-to-end by
		// TestARemovedTokenReachesAPlayerOnlyAsHidden (project_test.go).
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

// doorTransitions corrects what this viewer believes about the doors they can
// SEE, and is the scene-layer twin of the conditions that ride along with an
// actor's introduction.
//
// WHY IT HAS TO EXIST. engine.Scene.OpenDoors is not a field of anything the
// introduction already carries: the outline is a redacted SceneCreated and
// SceneSeen carries tiles and objects, so a door's OPEN state travels in
// neither. Without this, every door worked before a seat had eyes — every door
// in the onboarding gap between logging in and being assigned a character —
// stays shut on that player's board for the rest of the session. It never
// self-corrects, because the SceneCreated arms build OpenDoors EMPTY and only
// the two door arms ever write it afterwards, so a DoorOpened nobody sends is a
// door nothing will ever open. And it never throws, because both folds treat a
// repeat as idempotent. That combination is why it is found at a table rather
// than by CI.
//
// (Corrected 2026-08-21: this said OpenDoors was written "only by the two door
// arms", which reads past the initialisation in apply.go's and fold.ts's
// SceneCreated arms — the write that makes the door shut in the first place.)
//
// VISIBLE SQUARES ONLY, which is what makes this a correction rather than a
// leak. A door the viewer cannot see is left exactly as they last saw it — the
// same rule explored terrain follows — so this tells them nothing classify
// would have withheld.
//
// THE CAUSING EVENT IS SKIPPED, not suppressed. When the event being projected
// IS the door event for this square, classify forwards it whenever the square
// is visible, so emitting here as well would send two envelopes saying one
// thing. The belief is still recorded, so the next event does not re-emit. When
// the square is NOT visible the square is not walked at all, so the skip costs
// nothing in that direction.
func (pr *Projector) doorTransitions(cause *vttv1.Envelope, seq int64, now sightView, st *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	causeScene, causeSquare, causeIsDoor := doorSubject(cause)

	for _, id := range sortedSceneIDs(now.squares) {
		sc, ok := st.Scenes[id]
		if !ok {
			continue
		}
		believed := pr.doors[id]
		if believed == nil {
			believed = map[string]bool{}
			pr.doors[id] = believed
		}
		for _, sq := range sortedSet(now.squares[id]) {
			open := sc.OpenDoors[sq]
			if open == believed[sq] {
				// Covers every ordinary square too: a square with no door is
				// false on both sides and never reaches the emit below.
				continue
			}
			if causeIsDoor && id == causeScene && sq == causeSquare {
				believed[sq] = open
				continue
			}
			at, ok := squareAt(sq)
			if !ok {
				// Unreachable: sq came out of sight.VisibleFrom, which builds
				// every key through the same "%d,%d". Skipped rather than
				// guessed at, because a door at a position nobody can name is
				// not something to invent coordinates for.
				continue
			}
			if open {
				out = append(out, &vttv1.Envelope{Sequence: seq,
					Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{
						SceneId: id, At: at}}})
			} else {
				out = append(out, &vttv1.Envelope{Sequence: seq,
					Payload: &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{
						SceneId: id, At: at}}})
			}
			believed[sq] = open
		}
	}
	return out
}

// doorSubject reports the scene and square a door event is about, if env is
// one. Any other payload reports ok=false and names no square — and so does a
// NIL envelope, which is the perch path saying no event caused this at all
// (GetPayload is nil-safe, and the switch below has nothing to match).
func doorSubject(env *vttv1.Envelope) (sceneID, square string, ok bool) {
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_DoorOpened:
		at := p.DoorOpened.GetAt()
		return p.DoorOpened.GetSceneId(), squareKey(at.GetX(), at.GetY()), at != nil
	case *vttv1.Envelope_DoorClosed:
		at := p.DoorClosed.GetAt()
		return p.DoorClosed.GetSceneId(), squareKey(at.GetX(), at.GetY()), at != nil
	}
	return "", "", false
}

// squareAt is squareKey's inverse. It requires BOTH halves to be consumed
// entirely by strconv — unlike fmt.Sscanf's "%d,%d", which stops at the first
// non-digit and would read "1,1abc" as 1,1 — and reports ok=false rather than
// panicking, so a key that cannot be a square is skipped instead of becoming a
// door at coordinates nobody meant. Same reasoning as mapdef's parseSquareKey,
// which this deliberately mirrors.
func squareAt(sq string) (*vttv1.GridPosition, bool) {
	xs, ys, found := strings.Cut(sq, ",")
	if !found {
		return nil, false
	}
	x, err := strconv.ParseInt(xs, 10, 32)
	if err != nil {
		return nil, false
	}
	y, err := strconv.ParseInt(ys, 10, 32)
	if err != nil {
		return nil, false
	}
	return &vttv1.GridPosition{X: int32(x), Y: int32(y)}, true
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

// The three sorters below exist for one reason: Go randomises map iteration,
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

// sortedSceneIDsUnion is sortedSceneIDs over every id in EITHER map, each once.
//
// Its one caller needs to walk a scene that has left sight as well as those
// still in it, and those two facts live in two different maps: what the viewer
// was last told (pr.seen) and what they can see now (now.squares).
func sortedSceneIDsUnion(a, b map[string]map[string]bool) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, m := range []map[string]map[string]bool{a, b} {
		for k := range m {
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
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
