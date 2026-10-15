package gateway

import (
	"log/slog"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// Viewer is who a projection is for (SPEC-016).
type Viewer struct {
	ParticipantID string
	Role          identity.Role
	Viewpoint     string
}

// Projector is one connection's projection of the log, fed from its first event (SPEC-016).
type Projector struct {
	viewer Viewer
	// Append every frame sent here and nothing else: its fold is the memory (VTT-312).
	sent       []*vttv1.Envelope
	projection *perceive.Projection
}

func NewProjector(v Viewer) *Projector {
	return &Projector{viewer: v, projection: perceive.New(eyesFor(v), nil)}
}

// Ignore a player's Viewpoint: the client sets it, and honouring it lends a
// player an NPC's eyes (TestAPlayerCannotBorrowAnNpcsEyesByPerching).
func eyesFor(v Viewer) perceive.Eyes {
	switch v.Role {
	case identity.RolePlayer:
		return perceive.Eyes{Controller: v.ParticipantID}
	case identity.RoleSpectator:
		return perceive.Eyes{Shoulder: v.Viewpoint}
	}
	return perceive.Eyes{}
}

// Project returns the frames this viewer is sent for env, judged against st, the
// state after it. Never write to env (TestProjectingChangesNeitherTheEventNorTheState).
func (pr *Projector) Project(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	if env == nil {
		return nil
	}
	switch pr.viewer.Role {
	case identity.RoleDM, identity.RoleAgent:
		// Return env itself: the DM's and the agent's streams are the log.
		return []*vttv1.Envelope{env}
	case identity.RolePlayer, identity.RoleSpectator:
	default:
		// Send nothing to a role this build does not know (SPEC-016).
		return nil
	}
	if st == nil {
		return nil
	}
	shown, err := campaign.FoldPrefix(pr.sent)
	if err != nil {
		// Withhold the event when the viewer's fold fails (VTT-314).
		// #nosec G706 -- the sequence is an int64 given as a structured attribute.
		slog.Error("gateway: viewer fold failed; withholding event",
			"sequence", env.GetSequence(), "error", err)
		return nil
	}
	out := pr.projection.Event(eyesFor(pr.viewer), env, shown, st)
	pr.sent = append(pr.sent, out...)
	return out
}

// Keep perchSequence 0: a perch has no causing event, and a borrowed number
// names a frame no event caused (SPEC-015).
const perchSequence int64 = 0

// reperch moves this projection to actorID's shoulder (SPEC-015, SPEC-016).
func (pr *Projector) reperch(actorID string, st *engine.State) []*vttv1.Envelope {
	pr.viewer.Viewpoint = actorID
	if st == nil {
		// Keep this guard and look's (TestASeatPerchesOnlyAgainstAWorldItHasSeen).
		return nil
	}
	shown, err := campaign.FoldPrefix(pr.sent)
	if err != nil {
		slog.Error("gateway: viewer fold failed; withholding perch", "error", err)
		return nil
	}
	out := pr.projection.Perch(eyesFor(pr.viewer), shown, st, perchSequence)
	pr.sent = append(pr.sent, out...)
	return out
}
