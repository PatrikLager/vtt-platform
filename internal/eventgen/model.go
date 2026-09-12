// Package eventgen draws valid campaign histories for property tests.
//
// IT EMITS, IT DOES NOT APPEND, and that split is the whole reason the package
// exists. The model lived inside internal/campaign's property test until
// 2026-09-12, where every action both built an event AND appended it to a
// campaign, judging the response inline. A _test.go file cannot be imported, so
// internal/gateway's own property test could not reuse a line of it, and the
// alternative was a second generator that would disagree with this one within
// a month.
//
// A caller drives it with Step, applies the returned Action however it likes —
// campaign.Append for the rebuild-equals-live property, engine.Apply plus a
// Projector for the per-seat one — and reports the sequence back with
// Accepted. The model tracks only what it needs to choose a LEGAL target, and
// says with MustFail when it has deliberately chosen an illegal one.
package eventgen

import (
	"fmt"
	"math/rand"
	"slices"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Action is one drawn step.
//
// MustFail is the half that makes this a test and not a fuzzer: the model knows
// when it has aimed at something the engine is required to refuse, so a caller
// can assert the refusal rather than tolerate whatever happens. A guard nobody
// trips is a guard nobody has tested.
type Action struct {
	// Env is NIL when the draw found nothing legal to aim at — today only
	// removeActor, when every actor still holds a token. A caller must check:
	// passing a nil envelope to engine.Apply or campaign.Append fails two
	// layers away as "unknown event variant", which reads as a missing arm in
	// the fold rather than a draw that produced nothing.
	Env      *vttv1.Envelope
	Kind     string
	MustFail bool
	// Anchors says this event's sequence joins the pool narration anchors are
	// drawn from. Notes and narration deliberately do not — see Model.allSeqs.
	Anchors bool
}

// Model is the generator's belief about what is currently legal.
//
// It is NOT an oracle. Nothing here is ever compared against engine.State — the
// property tests compare states with their own comparison, and this only
// decides what to draw next. Keeping it out of the assertion is what stops a
// bug in the model from quietly becoming the expected answer.
type Model struct {
	// SessionID and ActorRole are stamped on every envelope, and whether they
	// matter depends entirely on what the caller does with it.
	//
	// UNDER campaign.Append THEY ARE INERT: stampSessionIDAgainst overwrites
	// SessionId unconditionally — a fresh id for SessionStarted, the open
	// session's id otherwise, "" when none is open — so the value here is never
	// honoured, and nothing in engine, campaign or store reads ActorRole at all.
	//
	// UNDER A CALLER DRIVING engine.Apply DIRECTLY they are not. That arm folds
	// SessionStarted as ID: env.SessionId, so leaving this at its default names
	// every session in the walk "sess-1", and a per-seat property would see
	// "dm" on events it means to have come from a player. They are fields
	// rather than constants so the second consumer this package was extracted
	// for can set them; the defaults keep campaign's walk exactly as it was.
	SessionID string
	ActorRole string

	scenes []string
	actors []string

	tokenIDs   []string
	tokenScene map[string]string
	tokenPos   map[string][2]int32
	tokenActor map[string]string // token -> the actor it stands for

	// openDoors is a SET, because engine.Apply's OpenDoors is one. A slice let
	// the same (x,y) be recorded twice when a walk opened a door it had already
	// opened, and one close then left the model believing a door was open that
	// the engine had deleted.
	openDoors  map[string]map[[2]int32]bool
	grants     map[string][]string
	conditions map[string][]string

	// allSeqs is the pool narration anchors are drawn from, and it can only be
	// filled by the caller: a sequence is assigned by whatever APPLIES an event,
	// so it comes back through Accepted. Notes and narration deliberately do not
	// join it — they are exercised as their own action kind rather than folded
	// into the pool a narration anchors into, which keeps their addition
	// independently verifiable against the pre-existing action mix.
	allSeqs []int64

	sessionOpen bool

	idN                           int
	sceneN, actorN, tokenN, noteN int

	noteKeys []string
}

// New returns a model believing nothing exists yet.
func New() *Model {
	return &Model{
		SessionID:  "sess-1",
		ActorRole:  "dm",
		tokenScene: map[string]string{},
		tokenPos:   map[string][2]int32{},
		tokenActor: map[string]string{},
		openDoors:  map[string]map[[2]int32]bool{},
		grants:     map[string][]string{},
		conditions: map[string][]string{},
	}
}

// Accepted reports that a drawn action was applied and given seq.
//
// THE MODEL OWNS THE ANCHOR POOL and cannot fill it alone: a sequence is
// assigned by whatever applied the event, so it has to come back. A caller that
// forgets this still produces valid walks, just ones where narration never
// anchors — which is why Anchors is on the Action rather than inferred here.
func (m *Model) Accepted(a Action, seq int64) {
	if a.Anchors {
		m.allSeqs = append(m.allSeqs, seq)
	}
}

func (m *Model) nextID() string {
	m.idN++
	return fmt.Sprintf("evt-%d", m.idN)
}

func (m *Model) env(payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: m.nextID(), SessionId: m.SessionID, ActorRole: m.ActorRole}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.ActorRemoved:
		e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.TokenRemoved:
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: p}
	case *vttv1.NarrationAdded:
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: p}
	case *vttv1.NoteDeleted:
		e.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: p}
	case *vttv1.DoorOpened:
		e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: p}
	case *vttv1.DoorClosed:
		e.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: p}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: p}
	case *vttv1.ActorControlRevoked:
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: p}
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
	case *vttv1.ConditionRemoved:
		e.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: p}
	default:
		// A NIL-PAYLOAD ENVELOPE FAILS TWO LAYERS AWAY as "unknown event
		// variant", which reads as a missing arm in the fold rather than a
		// missing case here. Measured 2026-09-12 in this package's ancestor.
		panic(fmt.Sprintf("eventgen: no case for payload %T — add one", payload))
	}
	return e
}

// Scenes, Actors and Tokens expose what the model believes exists, for callers
// that need to set up a viewer or assert against a seat's knowledge.
func (m *Model) Scenes() []string { return slices.Clone(m.scenes) }
func (m *Model) Actors() []string { return slices.Clone(m.actors) }
func (m *Model) Tokens() []string { return slices.Clone(m.tokenIDs) }

func (m *Model) canPlaceToken() bool { return len(m.scenes) > 0 && len(m.actors) > 0 }
func (m *Model) canMoveToken() bool  { return len(m.tokenIDs) > 0 }

// Step draws one valid action given what the model believes, and updates that
// belief for the draws it expects to succeed.
//
// THE BANDS are the thresholds below, on one uniform draw, in order: create
// scene [0, 0.05), add actor [0.05, 0.15), place token [0.15, 0.28) when a
// scene and an actor exist, move token [0.28, 0.52) when a token is on the
// board, add narration [0.52, 0.60), upsert note [0.60, 0.64), delete note
// [0.64, 0.68), then the four add/remove pairs — open door [0.68, 0.72), close
// door [0.72, 0.76), grant control [0.76, 0.79), revoke control [0.79, 0.82),
// apply condition [0.82, 0.86), remove condition [0.86, 0.91), remove token
// [0.91, 0.94), remove actor [0.94, 0.96) — and the remainder [0.96, 1.0)
// starts or ends a session, or adds an actor when one is already open.
//
// A GUARDED CASE HANDS ITS DRAW DOWNWARD rather than redrawing: when no token
// is on the board the [0.28, 0.52) draw falls to narration, and when the roster
// is empty the condition and control bands fall to remove token. That is why
// band width alone does not predict a count, and why remove condition is wider
// than the neighbours it is drawn beside.
//
// AN EXTRA rng DRAW MOVES THE WHOLE WALK. Changing which element an action
// picks is free; changing how many random numbers it consumes reshuffles every
// action after it. Measured: gating a preference behind one extra rng.Float64()
// took removeCondition from one draw per walk to zero and red the coverage
// guard, on logic that was correct.
func (m *Model) Step(rng *rand.Rand, idx int) Action {
	r := rng.Float64()
	switch {
	case r < 0.05:
		return m.sceneCreated()
	case r < 0.15:
		return m.addActor()
	case r < 0.28 && m.canPlaceToken():
		return m.placeToken(rng)
	case r < 0.52 && m.canMoveToken():
		return m.moveToken(rng)
	case r < 0.60:
		return m.addNarration(rng, idx)
	case r < 0.64:
		return m.upsertNote(rng, idx)
	case r < 0.68:
		return m.deleteNote(rng, idx)
	case r < 0.72 && len(m.scenes) > 0:
		return m.openDoor(rng, idx)
	case r < 0.76 && len(m.scenes) > 0:
		return m.closeDoor(rng)
	case r < 0.79 && len(m.actors) > 0:
		return m.grantControl(rng)
	case r < 0.82 && len(m.actors) > 0:
		return m.revokeControl(rng)
	case r < 0.86 && len(m.actors) > 0:
		return m.applyCondition(rng)
	case r < 0.91 && len(m.actors) > 0:
		return m.removeCondition(rng, idx)
	case r < 0.94:
		return m.removeToken(rng, idx)
	case r < 0.96 && len(m.actors) > 0:
		return m.removeActor(rng)
	default:
		switch {
		case !m.sessionOpen:
			return m.startSession()
		case rng.Float64() < 0.15:
			return m.endSession()
		default:
			return m.addActor()
		}
	}
}

// sceneCreated is NAMED FOR THE EVENT, unlike addActor/placeToken beside it,
// and the inconsistency is deliberate: it was createScene until 2026-09-02,
// when create_scene left the platform and there was no longer a command of that
// name for the label to mean. This model emits EVENTS and never issues a
// command, which is why the rename cost nothing; its siblings keep their
// command-shaped names because those commands still exist.
func (m *Model) sceneCreated() Action {
	m.sceneN++
	id := fmt.Sprintf("prop-scn-%d", m.sceneN)
	m.scenes = append(m.scenes, id)
	return Action{Env: m.env(&vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 20, GridHeight: 20,
	}), Kind: "sceneCreated", Anchors: true}
}

func (m *Model) addActor() Action {
	m.actorN++
	id := fmt.Sprintf("prop-actor-%d", m.actorN)
	m.actors = append(m.actors, id)
	return Action{Env: m.env(&vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id, ModuleId: "prop-module"},
	}), Kind: "addActor", Anchors: true}
}

// Int31n RATHER THAN int32(rng.Intn(...)) at every coordinate below, and it is
// not a style choice: gosec's G115 cannot prove an int -> int32 conversion
// safe, and silencing a linter to clear a finding is not something this repo
// does. Int31n returns an int32 so there is no conversion to judge. It consumes
// the same randomness — Intn calls Int31n for any n inside int32 — which is
// what lets the walk survive the change; measured, seed 1 is unmoved.
func (m *Model) placeToken(rng *rand.Rand) Action {
	m.tokenN++
	id := fmt.Sprintf("prop-tok-%d", m.tokenN)
	scene := m.scenes[rng.Intn(len(m.scenes))]
	actor := m.actors[rng.Intn(len(m.actors))]
	x, y := rng.Int31n(50), rng.Int31n(50)
	m.tokenIDs = append(m.tokenIDs, id)
	m.tokenScene[id] = scene
	m.tokenPos[id] = [2]int32{x, y}
	m.tokenActor[id] = actor
	return Action{Env: m.env(&vttv1.TokenPlaced{
		TokenId: id, SceneId: scene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}), Kind: "placeToken", Anchors: true}
}

func (m *Model) moveToken(rng *rand.Rand) Action {
	id := m.tokenIDs[rng.Intn(len(m.tokenIDs))]
	from := m.tokenPos[id]
	to := [2]int32{rng.Int31n(50), rng.Int31n(50)}
	scene := m.tokenScene[id]
	m.tokenPos[id] = to
	return Action{Env: m.env(&vttv1.TokenMoved{
		TokenId: id, SceneId: scene,
		From: &vttv1.GridPosition{X: from[0], Y: from[1]},
		To:   &vttv1.GridPosition{X: to[0], Y: to[1]},
	}), Kind: "moveToken", Anchors: true}
}

func (m *Model) startSession() Action {
	m.sessionOpen = true
	return Action{Env: m.env(&vttv1.SessionStarted{Name: "prop-session"}),
		Kind: "startSession", Anchors: true}
}

func (m *Model) endSession() Action {
	m.sessionOpen = false
	return Action{Env: m.env(&vttv1.SessionEnded{}), Kind: "endSession", Anchors: true}
}

// addNarration mixes anchored and unanchored draws: roughly half of every draw
// with at least two sequences in the pool anchors backward over a real range,
// respecting the spec's backward-only anchor rule.
func (m *Model) addNarration(rng *rand.Rand, idx int) Action {
	na := &vttv1.NarrationAdded{Text: fmt.Sprintf("narration entry #%d", idx)}
	if len(m.allSeqs) >= 2 && rng.Float64() < 0.5 {
		from := m.allSeqs[rng.Intn(len(m.allSeqs))]
		to := m.allSeqs[rng.Intn(len(m.allSeqs))]
		if from > to {
			from, to = to, from
		}
		na.AnchorFromSeq = from
		na.AnchorToSeq = to
	}
	return Action{Env: m.env(na), Kind: "addNarration"}
}

// upsertNote re-upserts an existing key about 30% of the time, exercising
// last-write-wins on the SAME key with no rejection expected.
func (m *Model) upsertNote(rng *rand.Rand, idx int) Action {
	var key string
	if len(m.noteKeys) > 0 && rng.Float64() < 0.30 {
		key = m.noteKeys[rng.Intn(len(m.noteKeys))]
	} else {
		m.noteN++
		key = fmt.Sprintf("prop-note-%d", m.noteN)
		m.noteKeys = append(m.noteKeys, key)
	}
	return Action{Env: m.env(&vttv1.NoteUpserted{
		Key: key, Title: fmt.Sprintf("Note %s", key),
		Text: fmt.Sprintf("text for %s at action #%d", key, idx),
	}), Kind: "upsertNote"}
}

// deleteNote aims at an absent key about 30% of the time, or whenever it tracks
// none at all, and says so with MustFail.
func (m *Model) deleteNote(rng *rand.Rand, idx int) Action {
	absent := len(m.noteKeys) == 0 || rng.Float64() < 0.30
	var key string
	if absent {
		key = fmt.Sprintf("prop-note-absent-%d", idx)
	} else {
		i := rng.Intn(len(m.noteKeys))
		key = m.noteKeys[i]
		m.noteKeys = append(m.noteKeys[:i], m.noteKeys[i+1:]...)
	}
	return Action{Env: m.env(&vttv1.NoteDeleted{Key: key}), Kind: "deleteNote", MustFail: absent}
}

// openDoor and closeDoor exercise the door pair, putting OpenDoors through
// whatever round-trip the caller applies. openDoor aims at an absent scene
// about 15% of the time: a guard nobody trips is a guard nobody has tested.
//
// closeDoor draws NO refusal, and inventing one would assert a rule the engine
// does not have — deleting a key that is not there is a documented no-op.
func (m *Model) openDoor(rng *rand.Rand, idx int) Action {
	unknown := rng.Float64() < 0.15
	scene := fmt.Sprintf("prop-scn-absent-%d", idx)
	if !unknown {
		scene = m.scenes[rng.Intn(len(m.scenes))]
	}
	x, y := rng.Int31n(20), rng.Int31n(20)
	if !unknown {
		if m.openDoors[scene] == nil {
			m.openDoors[scene] = map[[2]int32]bool{}
		}
		m.openDoors[scene][[2]int32{x, y}] = true
	}
	return Action{Env: m.env(&vttv1.DoorOpened{
		SceneId: scene, At: &vttv1.GridPosition{X: x, Y: y},
	}), Kind: "openDoor", MustFail: unknown, Anchors: !unknown}
}

func (m *Model) closeDoor(rng *rand.Rand) Action {
	scene := m.scenes[rng.Intn(len(m.scenes))]
	var at [2]int32
	if open := m.openDoors[scene]; len(open) > 0 && rng.Float64() < 0.8 {
		// SORTED, because ranging a map is unordered and this choice feeds the
		// walk: an unsorted pick would stop the same seed reproducing.
		keys := make([][2]int32, 0, len(open))
		for k := range open {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b [2]int32) int {
			if a[0] != b[0] {
				return int(a[0] - b[0])
			}
			return int(a[1] - b[1])
		})
		at = keys[rng.Intn(len(keys))]
		delete(open, at)
	} else {
		at = [2]int32{rng.Int31n(20), rng.Int31n(20)}
	}
	return Action{Env: m.env(&vttv1.DoorClosed{
		SceneId: scene, At: &vttv1.GridPosition{X: at[0], Y: at[1]},
	}), Kind: "closeDoor", Anchors: true}
}

// grantControl and revokeControl are tolerant about the control SET — a
// re-grant returns nil rather than erroring, and revoking from a participant
// who holds nothing filters an empty set — so neither draws a refusal here.
//
// THEY ARE NOT REFUSAL-FREE. Both enter the engine's controlTarget, which
// refuses an empty participant id and an unknown actor; this walk simply never
// supplies either, and those two guards are pinned by name in internal/engine's
// actor_control_test.
func (m *Model) grantControl(rng *rand.Rand) Action {
	actor := m.actors[rng.Intn(len(m.actors))]
	pid := fmt.Sprintf("prop-p-%d", rng.Intn(4))
	if held := m.grants[actor]; !slices.Contains(held, pid) {
		m.grants[actor] = append(held, pid)
	}
	return Action{Env: m.env(&vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: pid,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}), Kind: "grantControl", Anchors: true}
}

func (m *Model) revokeControl(rng *rand.Rand) Action {
	actor := m.actors[rng.Intn(len(m.actors))]
	pid := fmt.Sprintf("prop-p-%d", rng.Intn(4))
	if held := m.grants[actor]; len(held) > 0 && rng.Float64() < 0.8 {
		pid = held[rng.Intn(len(held))]
	}
	if held := m.grants[actor]; len(held) > 0 {
		m.grants[actor] = slices.DeleteFunc(slices.Clone(held), func(s string) bool { return s == pid })
	}
	return Action{Env: m.env(&vttv1.ActorControlRevoked{
		ActorId: actor, ParticipantId: pid,
	}), Kind: "revokeControl", Anchors: true}
}

// applyCondition and removeCondition are the strictest pair: the engine refuses
// a second application of a condition an actor already carries, and refuses
// removing one they do not. Both refusals are drawn.
func (m *Model) applyCondition(rng *rand.Rand) Action {
	actor := m.actors[rng.Intn(len(m.actors))]
	held := m.conditions[actor]
	duplicate := len(held) > 0 && rng.Float64() < 0.2
	cid := fmt.Sprintf("prop-cond-%d", rng.Intn(5))
	if duplicate {
		cid = held[rng.Intn(len(held))]
	} else if slices.Contains(held, cid) {
		duplicate = true // the random pick collided with one already applied
	}
	if !duplicate {
		m.conditions[actor] = append(held, cid)
	}
	return Action{Env: m.env(&vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cid, Source: "prop",
	}), Kind: "applyCondition", MustFail: duplicate, Anchors: !duplicate}
}

// removeCondition PREFERS AN ACTOR THAT ACTUALLY CARRIES SOMETHING, the way
// closeDoor prefers a door that is open: the pool is the actors carrying a
// condition when there are any, and every actor otherwise. Drawing uniformly
// made most draws hit an actor with nothing on them, turning them into the
// absent case.
//
// IT IS THE SMALLER OF TWO LEVERS. The pool alone moved the successful removal
// from one per walk to two, because it can only redistribute the draws this
// action GETS; widening its band from 0.03 to 0.05 is what moved it to seven.
//
// ONE rng.Intn EITHER WAY, deliberately — see Step on why an extra draw moves
// the whole walk.
func (m *Model) removeCondition(rng *rand.Rand, idx int) Action {
	pool := make([]string, 0, len(m.conditions))
	for a, carried := range m.conditions {
		if len(carried) > 0 {
			pool = append(pool, a)
		}
	}
	slices.Sort(pool) // see closeDoor: an unsorted pick stops the seed reproducing
	if len(pool) == 0 {
		pool = m.actors
	}
	actor := pool[rng.Intn(len(pool))]
	held := m.conditions[actor]
	absent := len(held) == 0 || rng.Float64() < 0.25
	cid := fmt.Sprintf("prop-cond-absent-%d", idx)
	if !absent {
		pick := rng.Intn(len(held))
		cid = held[pick]
		m.conditions[actor] = append(held[:pick], held[pick+1:]...)
	}
	return Action{Env: m.env(&vttv1.ConditionRemoved{
		ActorId: actor, ConditionId: cid, Reason: "prop",
	}), Kind: "removeCondition", MustFail: absent, Anchors: !absent}
}

// removeToken and removeActor take things out of the world, and carry the one
// CROSS-ENTITY rule the model has to know: engine.Apply refuses to remove an
// actor that still has a token on the board, because clearing the board is the
// command's job — internal/gateway's handleRemoveActor emits one TokenRemoved
// per token ahead of the ActorRemoved, as one batch — and cascading in the fold
// would put the same rule in two places.
//
// That rule is why the model tracks tokenActor at all. Both directions are
// drawn: an actor with no tokens must be accepted, and one still holding a
// token must be refused.
func (m *Model) removeToken(rng *rand.Rand, idx int) Action {
	absent := len(m.tokenIDs) == 0 || rng.Float64() < 0.2
	id := fmt.Sprintf("prop-tok-absent-%d", idx)
	if !absent {
		pick := rng.Intn(len(m.tokenIDs))
		id = m.tokenIDs[pick]
		m.tokenIDs = append(m.tokenIDs[:pick], m.tokenIDs[pick+1:]...)
		delete(m.tokenScene, id)
		delete(m.tokenPos, id)
		delete(m.tokenActor, id)
	}
	return Action{Env: m.env(&vttv1.TokenRemoved{TokenId: id}),
		Kind: "removeToken", MustFail: absent, Anchors: !absent}
}

func (m *Model) removeActor(rng *rand.Rand) Action {
	onBoard := map[string]bool{}
	for _, a := range m.tokenActor {
		onBoard[a] = true
	}
	// clean and held are built by walking m.actors, a SLICE, so the map above
	// only ever answers a membership question and never decides an order.
	var clean, held []string
	for _, a := range m.actors {
		if onBoard[a] {
			held = append(held, a)
		} else {
			clean = append(clean, a)
		}
	}
	blocked := len(held) > 0 && rng.Float64() < 0.25
	var id string
	switch {
	case blocked:
		id = held[rng.Intn(len(held))]
	case len(clean) > 0:
		id = clean[rng.Intn(len(clean))]
	default:
		// Nothing removable this step: every actor still holds a token, and
		// removing one is the refusal this action draws deliberately rather
		// than the outcome it wants. This is the ONLY arm that yields no
		// event, and unlike a guarded case in Step it is not handed downward —
		// the draw is simply spent.
		return Action{Kind: "removeActor"}
	}
	if !blocked {
		m.actors = slices.DeleteFunc(slices.Clone(m.actors), func(a string) bool { return a == id })
		delete(m.grants, id)
		delete(m.conditions, id)
	}
	return Action{Env: m.env(&vttv1.ActorRemoved{ActorId: id}),
		Kind: "removeActor", MustFail: blocked, Anchors: !blocked}
}
