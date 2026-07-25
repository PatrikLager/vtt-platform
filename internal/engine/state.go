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

type State struct {
	Scenes     map[string]Scene
	Actors     map[string]*vttv1.Actor
	Tokens     map[string]Token
	Sessions   []Session
	Conditions map[string][]ActorCondition // keyed by actor id
}

func NewState() *State {
	return &State{
		Scenes:     map[string]Scene{},
		Actors:     map[string]*vttv1.Actor{},
		Tokens:     map[string]Token{},
		Conditions: map[string][]ActorCondition{},
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
