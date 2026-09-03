package adventure

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// Compile turns adv into the deterministic, ordered list of setup envelopes
// (payloads only — sequence/session/participant/event_id/actor_role are
// stamped later by whatever calls campaign.AppendBatch with this list,
// exactly as gateway/convert.go's ToEvent leaves them for one-event
// commands) that one atomic AppendBatch will apply (spec §3/§5). Order is
// binding: AdventureLoaded (testimony, first — makes the log
// self-describing about what was loaded), scenes (file-name order), actors
// (file-name order), placements (scene order, then declared order within
// each scene), notes (declared order), and finally the opening narration
// (last).
//
// Every id/key adv declares is checked against st (the live campaign
// snapshot) for a collision BEFORE any envelope is built — spec §5:
// "checked against the live snapshot before the batch — rejection, not
// overwrite". A collision rejects the whole call; nothing is emitted.
//
// Compile carries no internal state and its only iteration over adv is
// positional (slices, in Load order) — it never ranges a Go map to decide
// output order — so two calls with the same (adv, st) always produce
// deep-equal results, warnings included.
//
// THE SECOND RETURN IS EVERY WARNING mapdef.BuildSceneCreated produced for any
// scene, each prefixed with the scene that produced it. handleLoadAdventure
// (internal/gateway/adventure.go) puts them on its CommandResult, the same
// channel handleLoadMap uses — spec §4's "the DM is told which references did
// not resolve, once, as a warning on the load — not an error, and not
// silence". Nothing else in this package interprets them.
func Compile(adv *Adventure, st *engine.State) ([]*vttv1.Envelope, []string, error) {
	if err := checkCollisions(adv, st); err != nil {
		return nil, nil, err
	}
	var warnings []string

	envs := make([]*vttv1.Envelope, 0, 1+len(adv.Scenes)+len(adv.Actors)+countPlacements(adv)+len(adv.Notes)+1)

	envs = append(envs, &vttv1.Envelope{
		Payload: &vttv1.Envelope_AdventureLoaded{AdventureLoaded: &vttv1.AdventureLoaded{
			AdventureId: adv.ID, Name: adv.Name,
		}},
	})

	for _, sc := range adv.Scenes {
		// Delegated, not built here: mapdef.BuildSceneCreated is the ONE
		// construction site a SceneCreated comes from, shared with the
		// standalone maps/ load path — see that function's own doc comment
		// (internal/mapdef/compile.go).
		//
		// ITS WARNINGS ARE CARRIED NOW, and the `_` that used to sit here was
		// a real silence rather than a tidy omission. Art-is-a-flat-library
		// Task 3 turned unresolvable art from a refusal into a warning, so
		// from that moment an adventure whose art/ was unopenable compiled
		// with err == nil, every square plain, and NOTHING said so anywhere —
		// which that design spec §4 names as the outcome strictly worse than
		// the refusal it replaced. Neither shipped adventure declares an
		// override or an object today, so nothing triggers it yet; Tasks 7-8
		// are building toward adventure art, and a silence that only appears
		// once the feature is used is the worst possible time to find it.
		created, w, err := mapdef.BuildSceneCreated(sc.asMap(), adv.ArtDir)
		if err != nil {
			return nil, warnings, fmt.Errorf("adventure: compile: scene %q: %w", sc.ID, err)
		}
		// Scene-qualified: mapdef deduplicates per SCENE, and an adventure
		// compiles several, so two scenes missing the same art would otherwise
		// produce two identical lines with nothing to tell them apart.
		for _, one := range w {
			warnings = append(warnings, fmt.Sprintf("scene %q: %s", sc.ID, one))
		}
		envs = append(envs, &vttv1.Envelope{
			Payload: &vttv1.Envelope_SceneCreated{SceneCreated: created},
		})
	}

	for _, a := range adv.Actors {
		envs = append(envs, &vttv1.Envelope{
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
				Actor: buildActor(a),
			}},
		})
	}

	for _, sc := range adv.Scenes {
		for _, p := range sc.Placements {
			envs = append(envs, &vttv1.Envelope{
				Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
					TokenId: p.TokenID, SceneId: sc.ID, ActorId: p.ActorID,
					Position: &vttv1.GridPosition{X: p.X, Y: p.Y},
				}},
			})
		}
	}

	for _, n := range adv.Notes {
		envs = append(envs, &vttv1.Envelope{
			Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
				Key: n.Key, Title: n.Title, Text: n.Text,
			}},
		})
	}

	envs = append(envs, &vttv1.Envelope{
		Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
			Text: adv.OpeningNarration,
		}},
	})

	return envs, warnings, nil
}

func countPlacements(adv *Adventure) int {
	n := 0
	for _, sc := range adv.Scenes {
		n += len(sc.Placements)
	}
	return n
}

// buildActor converts one AdventureActor into a fresh *vttv1.Actor,
// defensively copying its Attributes/Resources maps so the compiled
// envelope never aliases the loaded Adventure's own maps (mirrors
// engine.State.Snapshot's "readers never alias live state" discipline —
// engine/state.go). ControllerId is deliberately left unset: the DM drives
// every adventure-placed actor at load time (spec §4); a player can be
// given control later via the existing actor-control mechanism.
//
// KIND IS CARRIED, NOT DECIDED HERE. Load refuses an actor that does not
// declare one (loadActors), so by the time this runs the value is the
// author's own word and this function's only job is not to lose it. An
// earlier draft of this arc proposed having the compiler STAMP every
// adventure actor non-party instead; that would have dropped the Human
// Fighter, Hollis Ketch and Mara Voss out of their own party's roster on the
// only two adventures that exist.
func buildActor(a AdventureActor) *vttv1.Actor {
	attrs := make(map[string]int32, len(a.Attributes))
	for k, v := range a.Attributes {
		attrs[k] = v
	}
	res := make(map[string]*vttv1.Resource, len(a.Resources))
	for k, v := range a.Resources {
		res[k] = &vttv1.Resource{Current: v.Current, Max: v.Max}
	}
	return &vttv1.Actor{
		ActorId:    a.ID,
		Name:       a.Name,
		Kind:       a.Kind,
		Attributes: attrs,
		Resources:  res,
	}
}

// checkCollisions checks every id/key adv declares against st (the live
// snapshot) BEFORE any envelope is built, in the same order Compile would
// otherwise emit them (scenes, actors, tokens, notes) — deterministic, and
// consistent with Compile's own no-map-iteration discipline. Returns the
// first collision found, naming both the colliding id/key and its kind.
func checkCollisions(adv *Adventure, st *engine.State) error {
	for _, sc := range adv.Scenes {
		if _, ok := st.Scenes[sc.ID]; ok {
			return fmt.Errorf("adventure: compile: scene id %q collides with an existing scene in campaign state", sc.ID)
		}
	}
	for _, a := range adv.Actors {
		if _, ok := st.Actors[a.ID]; ok {
			return fmt.Errorf("adventure: compile: actor id %q collides with an existing actor in campaign state", a.ID)
		}
	}
	for _, sc := range adv.Scenes {
		for _, p := range sc.Placements {
			if _, ok := st.Tokens[p.TokenID]; ok {
				return fmt.Errorf("adventure: compile: token id %q collides with an existing token in campaign state", p.TokenID)
			}
		}
	}
	for _, n := range adv.Notes {
		if _, ok := st.Notes[n.Key]; ok {
			return fmt.Errorf("adventure: compile: note key %q collides with an existing note in campaign state", n.Key)
		}
	}
	return nil
}
