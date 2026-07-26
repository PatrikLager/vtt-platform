// Package engine holds the derived game state and the single fold that
// advances it. Apply is the only state mutator in the codebase (spec §3);
// the package does no I/O and imports only the contract.
package engine

import (
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

type Scene struct {
	ID, Name              string
	GridWidth, GridHeight int32
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
		out.Scenes[k] = v
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
