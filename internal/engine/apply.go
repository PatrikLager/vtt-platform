package engine

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

var ErrUnknownVariant = errors.New("engine: unknown event variant")

// Apply advances st by one event. It validates BEFORE mutating: any error
// return leaves st unchanged. AttackRolled, EventsRetracted, and AbilityUsed
// are deliberate no-ops here (spec §5).
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

	case *vttv1.Envelope_AbilityUsed:
		return nil // testimony, not state — meaning arrives via the ResourceChanged/ConditionApplied/ConditionRemoved events in the same batch (ruleset-interpreter spec §3)

	case *vttv1.Envelope_ResourceChanged:
		rc := p.ResourceChanged
		actor, ok := st.Actors[rc.ActorId]
		if !ok {
			return fmt.Errorf("engine: resource changed for unknown actor %q", rc.ActorId)
		}
		res, ok := actor.Resources[rc.Resource]
		if !ok {
			return fmt.Errorf("engine: resource changed for unknown resource %q on actor %q", rc.Resource, rc.ActorId)
		}
		// Compute the clamp in int64 so int32 arithmetic can never wrap
		// before the floor/cap is applied: a hand-crafted event could
		// otherwise drive res.Current + rc.Delta past int32, wrap to a value
		// the floor pulls to 0 (or the cap accepts), and get accepted with a
		// dishonest new_value (spec §3; matches internal/rules' int32-bounded
		// emission so a legitimately-emitted event still verifies exactly).
		computed := int64(res.Current) + int64(rc.Delta)
		if computed < 0 {
			computed = 0
		}
		if res.Max > 0 && computed > int64(res.Max) {
			computed = int64(res.Max)
		}
		// The interpreter computes the post-clamp value; the engine
		// verifies it here rather than trusting it, keeping the log's
		// testimony honest (spec §3).
		if computed != int64(rc.NewValue) {
			return fmt.Errorf("engine: resource %q on actor %q: event new_value %d does not match computed %d", rc.Resource, rc.ActorId, rc.NewValue, computed)
		}
		// computed == rc.NewValue (an int32), so it fits int32 for storage.
		actor.Resources[rc.Resource] = &vttv1.Resource{Current: int32(computed), Max: res.Max}
		return nil

	case *vttv1.Envelope_ConditionApplied:
		ca := p.ConditionApplied
		if _, ok := st.Actors[ca.ActorId]; !ok {
			return fmt.Errorf("engine: condition applied for unknown actor %q", ca.ActorId)
		}
		for _, c := range st.Conditions[ca.ActorId] {
			if c.ID == ca.ConditionId {
				return fmt.Errorf("engine: condition %q already applied to actor %q", ca.ConditionId, ca.ActorId)
			}
		}
		st.Conditions[ca.ActorId] = append(st.Conditions[ca.ActorId], ActorCondition{
			ID: ca.ConditionId, Source: ca.Source, AppliedSeq: env.Sequence,
		})
		return nil

	case *vttv1.Envelope_ConditionRemoved:
		cr := p.ConditionRemoved
		if _, ok := st.Actors[cr.ActorId]; !ok {
			return fmt.Errorf("engine: condition removed for unknown actor %q", cr.ActorId)
		}
		list := st.Conditions[cr.ActorId]
		idx := -1
		for i, c := range list {
			if c.ID == cr.ConditionId {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("engine: condition %q not present on actor %q", cr.ConditionId, cr.ActorId)
		}
		st.Conditions[cr.ActorId] = append(list[:idx], list[idx+1:]...)
		return nil

	default:
		return fmt.Errorf("%w: %T", ErrUnknownVariant, env.Payload)
	}
}
