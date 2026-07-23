package engine

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

var ErrUnknownVariant = errors.New("engine: unknown event variant")

// Apply advances st by one event. It validates BEFORE mutating: any error
// return leaves st unchanged. AttackRolled and EventsRetracted are deliberate
// no-ops here (spec §5).
func Apply(st *State, env *vttv1.Envelope) error {
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_SessionStarted:
		if st.openSession() >= 0 {
			return fmt.Errorf("engine: session already open")
		}
		st.Sessions = append(st.Sessions, Session{
			ID: env.SessionId, Name: p.SessionStarted.Name, StartSeq: env.Sequence,
		})
		return nil

	case *vttv1.Envelope_SessionEnded:
		i := st.openSession()
		if i < 0 {
			return fmt.Errorf("engine: no open session to end")
		}
		st.Sessions[i].EndSeq = env.Sequence
		return nil

	case *vttv1.Envelope_SceneCreated:
		sc := p.SceneCreated
		if _, dup := st.Scenes[sc.SceneId]; dup {
			return fmt.Errorf("engine: scene %q already exists", sc.SceneId)
		}
		st.Scenes[sc.SceneId] = Scene{
			ID: sc.SceneId, Name: sc.Name,
			GridWidth: sc.GridWidth, GridHeight: sc.GridHeight,
		}
		return nil

	case *vttv1.Envelope_ActorAdded:
		a := p.ActorAdded.Actor
		if a == nil || a.ActorId == "" {
			return fmt.Errorf("engine: actor_added requires an actor with an id")
		}
		if _, dup := st.Actors[a.ActorId]; dup {
			return fmt.Errorf("engine: actor %q already exists", a.ActorId)
		}
		st.Actors[a.ActorId] = proto.Clone(a).(*vttv1.Actor)
		return nil

	case *vttv1.Envelope_TokenPlaced:
		tp := p.TokenPlaced
		if _, dup := st.Tokens[tp.TokenId]; dup {
			return fmt.Errorf("engine: token %q already exists", tp.TokenId)
		}
		if _, ok := st.Scenes[tp.SceneId]; !ok {
			return fmt.Errorf("engine: token placed in unknown scene %q", tp.SceneId)
		}
		if _, ok := st.Actors[tp.ActorId]; !ok {
			return fmt.Errorf("engine: token placed for unknown actor %q", tp.ActorId)
		}
		if tp.Position == nil {
			return fmt.Errorf("engine: token placed without position")
		}
		st.Tokens[tp.TokenId] = Token{
			ID: tp.TokenId, SceneID: tp.SceneId, ActorID: tp.ActorId,
			X: tp.Position.X, Y: tp.Position.Y,
		}
		return nil

	case *vttv1.Envelope_TokenMoved:
		tm := p.TokenMoved
		tok, ok := st.Tokens[tm.TokenId]
		if !ok {
			return fmt.Errorf("engine: moved unknown token %q", tm.TokenId)
		}
		if tm.To == nil {
			return fmt.Errorf("engine: token move without destination")
		}
		tok.X, tok.Y = tm.To.X, tm.To.Y
		st.Tokens[tm.TokenId] = tok
		return nil

	case *vttv1.Envelope_AttackRolled:
		return nil // testimony, not state — rules meaning arrives in sub-project 5

	case *vttv1.Envelope_EventsRetracted:
		return nil // handled by campaign rebuild, not in-line

	default:
		return fmt.Errorf("%w: %T", ErrUnknownVariant, env.Payload)
	}
}
