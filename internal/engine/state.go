// Package engine holds the derived game state and the single fold that
// advances it. Apply is the only state mutator in the codebase (spec §3);
// the package does no I/O and imports only the contract.
package engine

import (
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Tile is one square's terrain: the engine's own translation of
// vttv1.TileRef, made at SceneCreated fold time so Blocked's spatial
// arithmetic never has to reach into the contract package. Kind is the
// closed spatial set "wall"/"floor"/"door" (maps-as-geometry spec §3.3) and
// is the only field Blocked reads. Material is opaque here — it is carried
// only to be handed back to a renderer or a ruleset later, never branched on
// (CLAUDE.md rule 5: no game-system vocabulary in platform code).
type Tile struct {
	Kind, Material, Art string
}

// SceneObject is scenery, never an actor: it does not act, move on its own,
// or hold state (maps-as-geometry spec §3.4). Kind is an OPEN descriptive
// label ("boulder", "chest") for the DM and the LLM to talk about — no
// behaviour may be inferred from it. Only BlocksSight and BlocksMove carry
// structural effect; Blocked reads BlocksMove. Copied out of
// vttv1.SceneObject at fold time, mirroring Tile.
type SceneObject struct {
	ObjectID        string
	Kind            string
	X, Y            int32
	Width, Height   int32
	RotationDegrees int32
	BlocksSight     bool
	BlocksMove      bool
	Art             string
}

type Scene struct {
	ID, Name              string
	GridWidth, GridHeight int32

	// Tiles, Objects and OpenDoors may all be nil/empty: a scene created
	// before maps-as-geometry, or one deliberately made without terrain, is
	// exactly what every scene was before this branch and must keep working
	// (Patrik's ruling 2026-08-13) — reads against a nil map or an empty
	// slice already return the zero value / iterate zero times in Go, so
	// Blocked needs no special case for "no terrain recorded" at all.
	Tiles   map[string]Tile // keyed "x,y", column then row (spec §4.1)
	Objects []SceneObject

	// OpenDoors tracks which door squares are currently open, keyed the
	// same way as Tiles. A key absent from this map means the door has
	// never been toggled, and Blocked reads that as CLOSED (see the
	// SceneCreated arm in apply.go) — the fail-closed direction: a door
	// wrongly shut is a puzzle, a door wrongly open is an ambush that does
	// not happen. Door state folds like everything else (spec §5), so
	// replay reconstructs it and undo works on it for free.
	//
	// INVARIANT: non-nil for any Scene the SceneCreated fold arm built.
	// Not guaranteed for a Scene assigned into st.Scenes some other way —
	// synthetic test tooling (internal/rules/conformance/conformance.go,
	// semgrep-exempted from the fold-only-writer rule as state that never
	// becomes a real campaign) does exactly that today, leaving it nil. The
	// DoorOpened/DoorClosed arms in apply.go guard against that: a nil map
	// would turn writing into a Go panic — crashing the fold instead of the
	// validation error Apply's own doc comment promises.
	OpenDoors map[string]bool

	// Explored is the squares this VIEWER has ever seen, keyed like Tiles.
	// Client-side memory mirrored here so the same fold runs in both
	// languages (visibility spec §6). It only ever grows: terrain is
	// remembered, creatures are not.
	//
	// EMPTY FOR THE DM AND FOR THE LOG. Nothing in a campaign's log produces
	// SceneSeen, so a Scene folded from the real log has Explored nil — this
	// field is populated only when folding a PROJECTION.
	//
	// `json:",omitempty"` is load-bearing, not decoration: Scene has no other
	// json tags (every other field always marshals, per this file's own
	// header comment), and cmd/vtt's dump plus internal/harness's golden
	// comparison both call json.Marshal(State) directly with no field
	// allowlist. Every existing scenarios/goldens/*/state.json fixture folds
	// from the real log, so Explored is nil there — an untagged field would
	// have serialized as `"Explored": null` on every one of them and failed
	// every golden byte-comparison this same commit is supposed to leave
	// green. Confirmed by running the golden suite both ways (see
	// task-3-report.md).
	Explored map[string]bool `json:",omitempty"`

	// Visible is the squares this VIEWER can see RIGHT NOW, keyed like Tiles,
	// and it is Explored's opposite number in every respect: REPLACED wholesale
	// by each SceneSeen rather than unioned, so it shrinks as freely as it
	// grows. The pair is what the client needs and neither half supplies alone —
	// terrain you remember but cannot currently see is `Explored − Visible`, and
	// that difference is the fog (visibility spec §6.1).
	//
	// NIL AND EMPTY MEAN DIFFERENT THINGS, which is why this field is never
	// initialised by the SceneCreated arm the way OpenDoors is. Nil is "no
	// SceneSeen has ever arrived for this scene" — the DM, the agent, and any
	// scene folded from the real log, none of which is a projection at all.
	// Empty is "a projection arrived and this seat can see nothing here". A
	// renderer that conflated them would blank the DM's board, so the
	// distinction is load-bearing rather than incidental, and client/src/state.ts
	// carries the same one as `undefined` versus `{}`.
	//
	// `json:",omitempty"` for exactly the reason Explored's carries it, and with
	// the same consequence: nil and empty both vanish from the dump, so the
	// goldens are untouched and the nil/empty distinction above lives in memory
	// only. The keystone (spec §4.3) compares dumps, so what it holds the two
	// folds to is the populated case.
	Visible map[string]bool `json:",omitempty"`
}

type Token struct {
	ID, SceneID, ActorID string
	X, Y                 int32
}

type Session struct {
	ID, Name         string
	StartSeq, EndSeq int64 // EndSeq 0 = open
}

// ActorCondition is a generic named marker attached to an actor. v1
// conditions carry no mechanical effect (ruleset-interpreter spec §4) —
// they are DM-narrated bookkeeping the engine tracks structurally only.
type ActorCondition struct {
	ID         string
	Source     string
	AppliedSeq int64
}

// Note is a durable, DM-authored world fact (world-layer spec §4):
// last-write-wins on the key; prior versions are not kept in State — they
// remain retrievable by replaying the log, free (NoteUpserted events are
// never mutated in place).
type Note struct {
	Title, Text string
	UpdatedSeq  int64
}

type State struct {
	Scenes     map[string]Scene
	Actors     map[string]*vttv1.Actor
	Tokens     map[string]Token
	Sessions   []Session
	Conditions map[string][]ActorCondition // keyed by actor id
	Notes      map[string]Note             // keyed by note key
}

func NewState() *State {
	return &State{
		Scenes:     map[string]Scene{},
		Actors:     map[string]*vttv1.Actor{},
		Tokens:     map[string]Token{},
		Conditions: map[string][]ActorCondition{},
		Notes:      map[string]Note{},
	}
}

// Snapshot returns a deep copy; readers never alias live state (spec §5).
func (st *State) Snapshot() *State {
	out := NewState()
	for k, v := range st.Scenes {
		// Scene now carries two maps (Tiles, OpenDoors). A plain `out.Scenes[k]
		// = v` would copy the Scene struct but alias the maps underneath it —
		// harmless for Tiles (never written after SceneCreated) but wrong for
		// OpenDoors, which DoorOpened/DoorClosed mutate later: a caller
		// holding this snapshot would see doors it never asked to see change
		// out from under it, exactly the aliasing this method's own doc
		// comment promises readers never get.
		tiles := make(map[string]Tile, len(v.Tiles))
		for tk, tv := range v.Tiles {
			tiles[tk] = tv
		}
		doors := make(map[string]bool, len(v.OpenDoors))
		for dk, dv := range v.OpenDoors {
			doors[dk] = dv
		}
		// Explored gets the same treatment as OpenDoors, for the same
		// reason: the SceneSeen fold arm mutates it after the Scene is
		// created, so a nil-check-then-share copy would let a snapshot
		// holder watch a projection's fog of war clear out from under it.
		// Unlike OpenDoors, nil stays nil here rather than becoming
		// `map[string]bool{}` — a DM/log-folded Scene's Explored is nil by
		// design (state.go's own doc comment on the field), and Snapshot
		// promises a deep COPY, not a change of zero value.
		var explored map[string]bool
		if v.Explored != nil {
			explored = make(map[string]bool, len(v.Explored))
			for ek, ev := range v.Explored {
				explored[ek] = ev
			}
		}
		// Visible copies exactly as Explored does, nil-preservation included —
		// but for a WEAKER reason, and the difference was measured rather than
		// assumed. Explored's copy is load-bearing today: the SceneSeen arm
		// writes into that map in place, so sharing it is observable. Visible's
		// is not, because the same arm REPLACES this map wholesale on every
		// message and never touches the one a snapshot took; substituting
		// `visible := v.Visible` here leaves the whole engine suite green (run,
		// not reasoned). It is copied anyway because Snapshot's promise is
		// unconditional — "readers never alias live state" — and the day
		// anything writes into Visible in place, the alias becomes a silent bug
		// rather than a caught one.
		//
		// Nil-preservation IS observable, and is pinned:
		// TestSnapshotOfUnprojectedSceneLeavesVisibleNil fails when nil is
		// promoted to empty, because that would tell a reader that a DM's scene
		// had received a projection reporting nothing visible (see the field's
		// own doc comment on why those two are not the same thing).
		var visible map[string]bool
		if v.Visible != nil {
			visible = make(map[string]bool, len(v.Visible))
			for vk, vv := range v.Visible {
				visible[vk] = vv
			}
		}
		out.Scenes[k] = Scene{
			ID: v.ID, Name: v.Name,
			GridWidth: v.GridWidth, GridHeight: v.GridHeight,
			Tiles:     tiles,
			Objects:   append([]SceneObject(nil), v.Objects...),
			OpenDoors: doors,
			Explored:  explored,
			Visible:   visible,
		}
	}
	for k, v := range st.Actors {
		out.Actors[k] = proto.Clone(v).(*vttv1.Actor)
	}
	for k, v := range st.Tokens {
		out.Tokens[k] = v
	}
	out.Sessions = append([]Session(nil), st.Sessions...)
	for k, v := range st.Conditions {
		out.Conditions[k] = append([]ActorCondition(nil), v...)
	}
	for k, v := range st.Notes {
		out.Notes[k] = v
	}
	return out
}

// openSession returns the index of the open session, or -1.
func (st *State) openSession() int {
	for i := range st.Sessions {
		if st.Sessions[i].EndSeq == 0 {
			return i
		}
	}
	return -1
}
