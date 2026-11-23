package perceive

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// sentAt projects log for eyes as a seat does, folding what it is sent, then
// folds change into the world and returns what the seat is sent for it.
func sentAt(t *testing.T, eyes Eyes, log []*vttv1.Envelope, change *vttv1.Envelope) []*vttv1.Envelope {
	t.Helper()
	world, shown := engine.NewState(), engine.NewState()
	pr := New(eyes, nil)
	for _, env := range append(append([]*vttv1.Envelope{}, log...), change) {
		if err := engine.Apply(world, env); err != nil {
			t.Fatalf("the world refuses seq %d: %v", env.GetSequence(), err)
		}
		sent := pr.Event(eyes, env, shown, world)
		for _, f := range sent {
			if err := engine.Apply(shown, f); err != nil {
				t.Fatalf("frame %T of seq %d does not fold: %v", f.GetPayload(), env.GetSequence(), err)
			}
		}
		if env == change {
			return sent
		}
	}
	return nil
}

// VTT-980 VTT-272
func TestEveryLogIsSentARulesetNamedWhateverItsEyesSee(t *testing.T) {
	far := append(abilityLog(), rogueGoesFar(13)...)
	for name, c := range map[string]struct {
		eyes Eyes
		log  []*vttv1.Envelope
	}{
		"eyes that see nothing":        {Eyes{}, abilityLog()},
		"a controller of no actor":     {Eyes{Controller: "p-9"}, abilityLog()},
		"a party member alone, far":    {Eyes{Shoulder: "rogue"}, far},
		"a party member beside others": {Eyes{Shoulder: "hero"}, abilityLog()},
	} {
		change := stamped(20, &vttv1.Envelope{Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{
			RulesetId: "qa-rules", Version: "1", Name: "Plain Rules"}}})[0]
		want := proto.Clone(change).(*vttv1.Envelope)
		want.ParticipantId, want.ActorRole = "", ""
		got := sentAt(t, c.eyes, c.log, change)
		if len(got) != 1 || !proto.Equal(got[0], want) {
			t.Errorf("%s is sent %v for the entry, want only %v", name, got, want)
		}
	}
}
