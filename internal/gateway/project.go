package gateway

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

// Viewer is who a projection is for: a participant, its role and, for a
// spectator, the shoulder it perches on (SPEC-016).
type Viewer struct {
	ParticipantID string
	Role          identity.Role
	Viewpoint     string
}

// Projector is one connection's projection of the log (SPEC-016).
// Feed it the log from the first event, never a snapshot of engine.State:
// its actors map remembers what left sight, and both folds refuse a second
// introduction (TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed).
type Projector struct {
	viewer Viewer

	// Never delete from scenes: a scene introduced twice is a fold error. Forget
	// an actor only in transitions' forgetting loop, after classify forwarded its
	// ActorRemoved, or in withdraw, which sends one (SPEC-016).
	scenes map[string]bool
	actors map[string]bool
	tokens map[string]bool
	// Compare seen, never merge it: SceneSeen carries the whole current set.
	seen map[string]map[string]bool
	// Correct a door belief only for a square in sight (doorTransitions): a
	// correction out of sight tells the viewer of a door it cannot see.
	doors map[string]map[string]bool
	// Forget a key only in noteTransitions, which sends its NoteDeleted: a key
	// forgotten anywhere else stays in the viewer's fold (SPEC-016).
	notes map[string]bool
	// Replace sighted only in snapshotSighted: classify judges every
	// actor-naming payload against it (SPEC-016).
	sighted map[string]bool
	// Write a belief only where the viewer's fold equals st (snapshotSighted).
	belief map[string]actorBelief
	// Withdraw a gone actor only before its id is introduced again: a bare
	// ActorRemoved at any other time reports a removal the viewer did not see.
	gone map[string]bool
}

func NewProjector(v Viewer) *Projector {
	return &Projector{
		viewer:  v,
		scenes:  map[string]bool{},
		actors:  map[string]bool{},
		tokens:  map[string]bool{},
		seen:    map[string]map[string]bool{},
		doors:   map[string]map[string]bool{},
		notes:   map[string]bool{},
		sighted: map[string]bool{},
		belief:  map[string]actorBelief{},
		gone:    map[string]bool{},
	}
}

// Pass these as arguments when a ruleset supplies them; never read a sight
// range off Actor.Attributes, which is game-system vocabulary (SPEC-016).
const (
	sightRangeNotSupplied int32 = 0
	toleranceNotSupplied  int   = 0
)

// Project returns the frames this viewer is sent for env, judged against st,
// the state after it (SPEC-016). Never write to env or to anything it points
// at: a live event is one envelope shared by every seat
// (TestProjectingChangesNeitherTheEventNorTheState).
func (pr *Projector) Project(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	if env == nil {
		return nil
	}
	switch pr.viewer.Role {
	case identity.RoleDM, identity.RoleAgent:
		// Return env itself: the DM's and the agent's streams are the log
		// (TestTheDMReceivesEverythingUnchanged).
		return []*vttv1.Envelope{env}
	case identity.RolePlayer, identity.RoleSpectator:
	default:
		// Send nothing to a role this build does not know (SPEC-016).
		return nil
	}
	if st == nil {
		return nil
	}

	now := pr.look(st)
	v := pr.classify(env, now)
	if v == unrecognised {
		// Emit nothing for a payload classify does not know, not even transitions:
		// what it did to the world is unknown (SPEC-016).
		return nil
	}
	out := pr.transitions(env, env.GetSequence(), now, st)
	if v == forwarded {
		out = append(out, forwardable(env))
	}
	return out
}

// perchSequence is the sequence of every frame a perch sends (SPEC-015).
// Keep it 0: a perch has no causing event, and a borrowed number names a
// frame no event caused.
const perchSequence int64 = 0

// reperch is transitions with no causing event, for the perch SPEC-015
// applies (SPEC-016). Pass no cause: doorTransitions then skips no door, and
// new eyes need every door they see corrected
// (TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen).
func (pr *Projector) reperch(actorID string, st *engine.State) []*vttv1.Envelope {
	pr.viewer.Viewpoint = actorID
	if st == nil {
		// Keep this guard and look's: with neither, a perch on a seat that has
		// folded nothing panics (TestASeatPerchesOnlyAgainstAWorldItHasSeen).
		return nil
	}
	return pr.transitions(nil, perchSequence, pr.look(st), st)
}

// sightView is one look at the world through this viewer's eyes (SPEC-016).
type sightView struct {
	squares map[string]map[string]bool
	tokens  map[string]bool
	actors  map[string]bool
	sees    map[string]bool
}

// look is what this viewer sees and may know of st, recomputed on every call
// (SPEC-016). Add no memo that skips events: deciding which events cannot
// change sight is where a leak would hide.
func (pr *Projector) look(st *engine.State) sightView {
	v := sightView{
		squares: map[string]map[string]bool{},
		tokens:  map[string]bool{},
		actors:  map[string]bool{},
		sees:    map[string]bool{},
	}
	if st == nil {
		return v
	}
	for _, eye := range pr.eyes(st) {
		v.actors[eye] = true
		v.sees[eye] = true
		// Walk this map unordered only because the loop unions a set; sort every
		// walk that emits frames (sortedSet).
		for _, tok := range st.Tokens {
			if tok.ActorID != eye {
				continue
			}
			sc, ok := st.Scenes[tok.SceneID]
			if !ok {
				continue
			}
			// Create the scene's entry before asking sight: standing in a scene earns
			// its board even when nothing is visible from there.
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
		if v.squares[tok.SceneID][squareKey(tok.X, tok.Y)] {
			v.tokens[id] = true
			v.actors[tok.ActorID] = true
			v.sees[tok.ActorID] = true
		}
	}
	for id, a := range st.Actors {
		// Know every party member, seen or not (SPEC-016).
		if engine.IsPartyMember(a) {
			v.actors[id] = true
		}
	}
	return v
}

// eyes are the actors this viewer sees through (SPEC-016).
func (pr *Projector) eyes(st *engine.State) []string {
	switch pr.viewer.Role {
	case identity.RolePlayer:
		// Ignore Viewpoint for a player: it comes from the client, and honouring it
		// lends a player an NPC's eyes (TestAPlayerCannotBorrowAnNpcsEyesByPerching).
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
		// Refuse a shoulder that is not a party member here too, with MayPerch's
		// predicate, so the two refusals cannot drift (SPEC-015).
		a, ok := st.Actors[pr.viewer.Viewpoint]
		if !ok || !engine.IsPartyMember(a) {
			return nil
		}
		return []string{pr.viewer.Viewpoint}
	}
	return nil
}

// transitions are the frames that bring this viewer's board from what it
// was sent to what it sees now, each carrying seq (SPEC-016). Keep each
// scene before the doors, tokens and SceneSeen in it, and each actor before
// its grants, conditions and tokens: both folds refuse them the other way
// round. cause is nil on a perch (reperch).
func (pr *Projector) transitions(cause *vttv1.Envelope, seq int64, now sightView, st *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope

	// Forget an actor the world no longer has only when this seat was forwarded
	// its ActorRemoved, after classify and before any introduction; keep one
	// removed unseen as gone (SPEC-016).
	for id := range pr.actors {
		if _, ok := st.Actors[id]; ok {
			continue
		}
		if pr.sighted[id] {
			delete(pr.actors, id)
			delete(pr.belief, id)
		} else {
			pr.gone[id] = true
		}
	}

	for _, id := range sortedSceneIDs(now.squares) {
		if pr.scenes[id] {
			continue
		}
		sc := st.Scenes[id]
		// Introduce the outline alone: tiles and objects arrive in SceneSeen.
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
				SceneId:    sc.ID,
				Name:       sc.Name,
				GridWidth:  sc.GridWidth,
				GridHeight: sc.GridHeight,
			}}})
		pr.scenes[id] = true
	}

	// Correct doors only after the scenes are introduced: both folds refuse a
	// door in a scene they do not have.
	out = append(out, pr.doorTransitions(cause, seq, now, st)...)

	for _, id := range sortedSet(now.actors) {
		a, ok := st.Actors[id]
		if !ok {
			continue
		}
		if pr.gone[id] {
			out = append(out, pr.withdraw(id, seq))
		}
		if !pr.actors[id] {
			out = append(out, pr.introduce(id, seq, a, st)...)
			continue
		}
		if now.sees[id] && !pr.sighted[id] {
			out = append(out, pr.correct(id, seq, a, st)...)
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

	// Walk the scenes last reported as well as those in sight: a scene with no
	// eye left is reported dark once, or the client keeps it lit
	// (TestASceneThatLeavesSightEntirelyIsReportedDark).
	for _, id := range sortedSceneIDsUnion(pr.seen, now.squares) {
		// Read st.Scenes unguarded: every id here came from st, and nothing
		// removes a scene.
		sc := st.Scenes[id]
		lit, inSight := now.squares[id]
		if sameSet(pr.seen[id], lit) {
			continue
		}
		// Send an empty SceneSeen when the scene goes dark: it darkens the view and
		// forgets no terrain (TestAnEmptySceneSeenDarkensTheSceneAndForgetsNoTerrain).
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_SceneSeen{SceneSeen: sceneSeenFor(sc, lit)}})
		if !inSight {
			// Forget the scene once it is reported dark, or the empty SceneSeen repeats
			// (TestASceneAlreadyReportedDarkIsNotReportedDarkAgain).
			delete(pr.seen, id)
			continue
		}
		pr.seen[id] = lit
	}

	pr.snapshotSighted(now, st)
	return append(out, pr.noteTransitions(cause, seq, st)...)
}

// sceneSeenFor is the whole of what this viewer sees of sc now, never a
// delta (SPEC-016).
func sceneSeenFor(sc engine.Scene, squares map[string]bool) *vttv1.SceneSeen {
	// Send the square set itself, sorted (SPEC-016): tiles lose every square with
	// no terrain, and an unsorted walk changes the bytes between runs.
	ss := &vttv1.SceneSeen{
		SceneId: sc.ID,
		Tiles:   map[string]*vttv1.TileRef{},
		Visible: sortedSet(squares),
	}
	for sq := range squares {
		t, ok := sc.Tiles[sq]
		if !ok {
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

// objectInSight reports whether any square of o's footprint is visible
// (SPEC-016). Keep the walk clamped to the grid and compared in int64: an
// enormous footprint otherwise spins, and an overflowing one wraps.
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

// verdict is classify's ruling on one payload (SPEC-016).
type verdict int

const (
	unrecognised verdict = iota
	withheld
	forwarded
)

func passIf(ok bool) verdict {
	if ok {
		return forwarded
	}
	return withheld
}

// classify rules on env for this viewer (SPEC-016). Keep an arm for every
// payload and the default unrecognised: a default that forwards leaks
// (TestEveryEnvelopePayloadArmHasAnExplicitRuling). Call it before
// transitions, which moves pr.tokens, pr.actors and pr.sighted past this event.
func (pr *Projector) classify(env *vttv1.Envelope, now sightView) verdict {
	saw := func(ids ...string) bool {
		for _, id := range ids {
			if id != "" && !pr.sighted[id] {
				return false
			}
		}
		return true
	}

	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_SessionStarted, *vttv1.Envelope_SessionEnded:
		return forwarded

	case *vttv1.Envelope_NarrationAdded:
		// Forward narration to every viewer: it is addressed to the table.
		return forwarded

	case *vttv1.Envelope_SceneCreated:
		// Withhold what transitions introduces: a second path to one introduction
		// sends a duplicate that both folds refuse (SPEC-016).
		return withheld

	case *vttv1.Envelope_ActorAdded:
		return withheld

	case *vttv1.Envelope_TokenPlaced:
		return withheld

	case *vttv1.Envelope_TokenRemoved:
		// Withhold: a viewer that held the token is sent a TokenHidden
		// (TestARemovedTokenReachesAPlayerOnlyAsHidden).
		return withheld

	case *vttv1.Envelope_TokenHidden, *vttv1.Envelope_SceneSeen:
		// Withhold: only the projection issues these.
		return withheld

	case *vttv1.Envelope_NoteUpserted:
		return passIf(p.NoteUpserted.GetVisibility() == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC)

	case *vttv1.Envelope_NoteDeleted:
		// Withhold: noteTransitions sends a holder the bare NoteDeleted a note
		// made secret gets, so the two cannot be told apart (SPEC-016).
		return withheld

	case *vttv1.Envelope_AdventureLoaded:
		// Withhold: the load names the adventure, and each event of its batch is
		// projected on its own.
		return withheld

	case *vttv1.Envelope_TokenMoved:
		// Require the token held before and seen after: a move names both ends
		// (TestSteppingIntoViewArrivesRatherThanMoves).
		id := p.TokenMoved.GetTokenId()
		return passIf(pr.tokens[id] && now.tokens[id])

	case *vttv1.Envelope_DoorOpened:
		return passIf(pr.canSeeSquare(now, p.DoorOpened.GetSceneId(), p.DoorOpened.GetAt()))

	case *vttv1.Envelope_DoorClosed:
		return passIf(pr.canSeeSquare(now, p.DoorClosed.GetSceneId(), p.DoorClosed.GetAt()))

	// Forward these when the viewer saw every actor they name before the event;
	// an actor it comes to see is corrected in transitions (SPEC-016).

	case *vttv1.Envelope_AttackRolled:
		return passIf(saw(p.AttackRolled.GetAttackerId(), p.AttackRolled.GetTargetId()))

	case *vttv1.Envelope_AbilityUsed:
		return passIf(saw(append([]string{p.AbilityUsed.GetActorId()},
			p.AbilityUsed.GetTargetIds()...)...))

	case *vttv1.Envelope_ActorControlGranted:
		return passIf(saw(p.ActorControlGranted.GetActorId()))

	case *vttv1.Envelope_ActorControlRevoked:
		return passIf(saw(p.ActorControlRevoked.GetActorId()))

	case *vttv1.Envelope_ResourceChanged:
		return passIf(saw(p.ResourceChanged.GetActorId()))

	case *vttv1.Envelope_ConditionApplied:
		return passIf(saw(p.ConditionApplied.GetActorId()))

	case *vttv1.Envelope_ConditionRemoved:
		return passIf(saw(p.ConditionRemoved.GetActorId()))

	case *vttv1.Envelope_ActorRemoved:
		// Forward only to a seat that saw the actor before the event; one that did
		// not keeps it until withdraw (SPEC-016).
		return passIf(saw(p.ActorRemoved.GetActorId()))

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

// doorTransitions corrects what this viewer believes of the doors it sees
// now (SPEC-016). Skip the square of the door event being projected:
// classify forwards that event, and a second frame would repeat it.
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
				continue
			}
			if causeIsDoor && id == causeScene && sq == causeSquare {
				believed[sq] = open
				continue
			}
			at, ok := squareAt(sq)
			if !ok {
				// Skip a key that names no square rather than invent coordinates.
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

// doorSubject reports the scene and square a door event is about, and none for
// a perch's nil cause (SPEC-016).
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

// squareAt is squareKey's inverse, refusing a key whose halves strconv
// does not consume whole.
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

// squareKey must build the key sight.VisibleFrom builds, or no token and no
// door on a visible square is ever seen.
func squareKey(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// Sort every set before emitting from it: map order is random, and one log
// must project to the same stream on every run
// (TestTheSameLogProjectsTheSameStreamEveryTime).

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

// sortedSceneIDsUnion is sortedSceneIDs over every id in either map, once.
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

// noteTransitions keeps this viewer's notes to the public ones (SPEC-016).
func (pr *Projector) noteTransitions(cause *vttv1.Envelope, seq int64, st *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, key := range sortedSet(pr.notes) {
		n, ok := st.Notes[key]
		if ok && n.Visibility == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			continue
		}
		delete(pr.notes, key)
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}})
	}
	if nu := cause.GetNoteUpserted(); nu.GetVisibility() == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
		pr.notes[nu.GetKey()] = true
	}
	return out
}

// introduce sends the viewer an actor it does not hold, with its controllers
// as grants and its conditions behind it (SPEC-016).
func (pr *Projector) introduce(id string, seq int64, a *vttv1.Actor, st *engine.State) []*vttv1.Envelope {
	// Clone the actor: st.Actors holds live pointers a later grant mutates.
	clone := proto.Clone(a).(*vttv1.Actor)

	// Clear the controllers from the introduction, since both folds refuse an
	// ActorAdded that names one, and send each behind it as a grant stating the
	// actor's kind (TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt).
	controllers := clone.GetControllerIds()
	clone.ControllerId = ""
	clone.ControllerIds = nil
	out := []*vttv1.Envelope{{Sequence: seq,
		Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: clone}}}}
	for _, cid := range controllers {
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: id, ParticipantId: cid, Kind: a.GetKind()}}})
	}
	// Send its conditions behind it, since the Actor does not carry them, and
	// without their source (VTT-249).
	for _, c := range st.Conditions[id] {
		out = append(out, &vttv1.Envelope{Sequence: seq,
			Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
				ActorId: id, ConditionId: c.ID}}})
	}
	pr.actors[id] = true
	pr.belief[id] = beliefOf(a, st.Conditions[id])
	return out
}

// withdraw forgets a held actor and returns the bare ActorRemoved that takes it
// out of the viewer's fold, which both folds accept only while no token of it
// is on the viewer's board: send it only before an introduction (SPEC-016).
func (pr *Projector) withdraw(id string, seq int64) *vttv1.Envelope {
	delete(pr.actors, id)
	delete(pr.belief, id)
	delete(pr.gone, id)
	return &vttv1.Envelope{Sequence: seq,
		Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}}
}

// actorBelief is what the viewer's fold holds of an actor that an event can
// change: each resource's current value, its conditions, its controllers and
// its kind (SPEC-016).
type actorBelief struct {
	resources   map[string]int32
	conditions  []string
	controllers []string
	kind        vttv1.ActorKind
}

func beliefOf(a *vttv1.Actor, conditions []engine.ActorCondition) actorBelief {
	b := actorBelief{resources: map[string]int32{}, kind: a.GetKind()}
	for name, r := range a.GetResources() {
		b.resources[name] = r.GetCurrent()
	}
	for _, c := range conditions {
		b.conditions = append(b.conditions, c.ID)
	}
	b.controllers = append(b.controllers, a.GetControllerIds()...)
	return b
}

// snapshotSighted records the belief of every held actor this event's frames
// left equal to st: one seen before the event, or seen now (SPEC-016).
func (pr *Projector) snapshotSighted(now sightView, st *engine.State) {
	for _, set := range []map[string]bool{pr.sighted, now.sees} {
		for id := range set {
			if a, ok := st.Actors[id]; ok && pr.actors[id] {
				pr.belief[id] = beliefOf(a, st.Conditions[id])
			}
		}
	}
	pr.sighted = now.sees
}

// correct brings the viewer's fold of a held actor that comes into sight to
// its present status, in frames carrying seq and nothing that says who or what
// changed it (SPEC-016).
func (pr *Projector) correct(id string, seq int64, a *vttv1.Actor, st *engine.State) []*vttv1.Envelope {
	b := pr.belief[id]
	fix, ok := statusFrames(id, seq, b, a, st.Conditions[id])
	if !ok {
		// Re-introduce what no grant can carry.
		return append([]*vttv1.Envelope{pr.withdraw(id, seq)}, pr.introduce(id, seq, a, st)...)
	}
	pr.belief[id] = beliefOf(a, st.Conditions[id])
	return fix
}

// statusFrames are the frames that bring belief b to actor a, or false when
// none can: a resource set that differs, or a kind no held controller carries
// (SPEC-016).
func statusFrames(id string, seq int64, b actorBelief, a *vttv1.Actor, conditions []engine.ActorCondition) ([]*vttv1.Envelope, bool) {
	var out []*vttv1.Envelope
	frame := func(e *vttv1.Envelope) {
		e.Sequence = seq
		out = append(out, e)
	}

	if len(a.GetResources()) != len(b.resources) {
		return nil, false
	}
	names := make([]string, 0, len(a.GetResources()))
	for name := range a.GetResources() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		was, held := b.resources[name]
		if !held {
			return nil, false
		}
		cur := a.GetResources()[name].GetCurrent()
		if cur == was {
			continue
		}
		// Pass through zero when the difference does not fit int32: the fold floors
		// a negative current at 0 (TestAResourceStartedBelowZeroIsCorrectedOnSight).
		if d := int64(cur) - int64(was); d > math.MaxInt32 {
			frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
				ActorId: id, Resource: name}}})
			was = 0
		}
		frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
			ActorId: id, Resource: name, Delta: cur - was, NewValue: cur}}})
	}

	present := map[string]bool{}
	for _, c := range conditions {
		present[c.ID] = true
	}
	believed := map[string]bool{}
	for _, c := range b.conditions {
		believed[c] = true
		if !present[c] {
			frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
				ActorId: id, ConditionId: c}}})
		}
	}
	for _, c := range conditions {
		if !believed[c.ID] {
			frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
				ActorId: id, ConditionId: c.ID}}})
		}
	}

	controls := map[string]bool{}
	for _, c := range a.GetControllerIds() {
		controls[c] = true
	}
	held := map[string]bool{}
	for _, c := range b.controllers {
		held[c] = true
	}
	grant := func(pid string) {
		frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: id, ParticipantId: pid, Kind: a.GetKind()}}})
	}
	granted := false
	for _, c := range a.GetControllerIds() {
		if !held[c] {
			grant(c)
			granted = true
		}
	}
	if a.GetKind() != b.kind && !granted {
		// Re-state a controller the fold holds: a grant is the one frame that
		// carries a kind, and naming any other would tell of a participant unseen.
		if len(b.controllers) == 0 || a.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			return nil, false
		}
		grant(b.controllers[0])
	}
	for _, c := range b.controllers {
		if !controls[c] {
			frame(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
				ActorId: id, ParticipantId: c}}})
		}
	}
	return out, true
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

// forwardable is a copy of env without its issuer or any cause (SPEC-016).
// Clone before clearing: every seat and the DM share env.
func forwardable(env *vttv1.Envelope) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	c.ParticipantId = ""
	c.ActorRole = ""
	switch p := c.GetPayload().(type) {
	case *vttv1.Envelope_TokenMoved:
		p.TokenMoved.Reason = ""
	case *vttv1.Envelope_ResourceChanged:
		p.ResourceChanged.Reason = ""
	case *vttv1.Envelope_ConditionApplied:
		p.ConditionApplied.Source = ""
	case *vttv1.Envelope_ConditionRemoved:
		p.ConditionRemoved.Reason = ""
	}
	return c
}
