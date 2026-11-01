package perceive

import (
	"slices"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// whisperWorld holds three party members: kim controls brann and tobbe, ann
// controls lera.
func whisperWorld(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	seq := int64(0)
	apply := func(env *vttv1.Envelope) {
		seq++
		env.EventId, env.Sequence = "e", seq
		if err := engine.Apply(st, env); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"brann", "lera", "tobbe"} {
		apply(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: party}}}})
	}
	for id, who := range map[string]string{"brann": "p-kim", "tobbe": "p-kim", "lera": "p-ann"} {
		apply(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: id, ParticipantId: who, Kind: party}}})
	}
	return st
}

func whisperFrames(pr *Projection, eyes Eyes, w *vttv1.WhisperAdded, st *engine.State) []*vttv1.WhisperAdded {
	env := &vttv1.Envelope{EventId: "w", Sequence: 9, ActorRole: "dm", ParticipantId: "p-dm",
		Payload: &vttv1.Envelope_WhisperAdded{WhisperAdded: w}}
	var out []*vttv1.WhisperAdded
	for _, f := range pr.Event(eyes, env, engine.NewState(), st) {
		if got := f.GetWhisperAdded(); got != nil {
			out = append(out, got)
		}
	}
	return out
}

// VTT-584
func TestAWhisperIsForwardedThroughEitherKindOfEyes(t *testing.T) {
	st := whisperWorld(t)
	for name, c := range map[string]struct {
		eyes Eyes
		want []string
	}{
		"a shoulder":   {Eyes{Shoulder: "tobbe"}, []string{"tobbe"}},
		"a controller": {Eyes{Controller: "p-kim"}, []string{"brann", "tobbe"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := whisperFrames(New(c.eyes, st), c.eyes, &vttv1.WhisperAdded{
				Text: "psst", ToActorIds: []string{"brann", "lera", "tobbe"}}, st)
			if len(got) != 1 || !slices.Equal(got[0].GetToActorIds(), c.want) {
				t.Fatalf("forwarded %v, want one whisper to %v", got, c.want)
			}
			if got := whisperFrames(New(c.eyes, st), c.eyes, &vttv1.WhisperAdded{
				Text: "psst", ToActorIds: []string{"lera"}}, st); len(got) != 0 {
				t.Fatalf("a whisper to lera alone was forwarded: %v", got)
			}
			got = whisperFrames(New(c.eyes, st), c.eyes, &vttv1.WhisperAdded{
				Text: "I saw it", FromActorId: "tobbe"}, st)
			if len(got) != 1 || got[0].GetFromActorId() != "tobbe" {
				t.Fatalf("tobbe's whisper to the DM forwarded as %v", got)
			}
		})
	}
}
