package gateway_test

import (
	"errors"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func cellarMove(requestID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "tok-fighter", To: &vttv1.GridPosition{X: x, Y: y}}}}
}

// VTT-602
func TestAPlayersMoveOntoWhatTheySeeIsRefusedInPlainWords(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellar(t)
	player := f.dialOn(f.playerToken, 0, "act-fighter")
	for _, c := range []struct {
		id   string
		x, y int32
		want string
	}{
		{"onto-the-wall", 2, 0, "You cannot move there: a wall."},
		{"onto-the-door", 0, 1, "You cannot move there: a closed door."},
		{"onto-the-boulder", 1, 1, "You cannot move there: something (a boulder) is in the way."},
	} {
		if got := f.refusedUnmoved(t, player, cellarMove(c.id, c.x, c.y)); got != c.want {
			t.Fatalf("%s was refused with %q, want %q", c.id, got, c.want)
		}
	}
	dm := f.dial(f.dmToken, f.head(t))
	accepted(t, dm, &vttv1.ClientCommand{RequestId: "open-the-door", Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: "cellar", At: &vttv1.GridPosition{X: 0, Y: 1}}}})
	accepted(t, player, cellarMove("through-the-door", 0, 1))
}

// VTT-603
func TestANamelessCharacterIsToldInPlainWordsWhyItCannotSpeakOrWhisper(t *testing.T) {
	const line = "This character has no name. Ask the DM for one, or speak out of character with /ooc."
	const whisper = "This character has no name. Ask the DM for one with /ooc, then whisper."
	say := func(requestID, actorID string) *vttv1.ClientCommand {
		cmd := sayCmd(actorID, "", false)
		cmd.RequestId = requestID
		return cmd
	}
	f := newGWFixture(t)
	f.whisperTable(t)
	lera := f.dial(f.playerToken, 0)
	if got := f.refusedUnmoved(t, lera, say("nameless-line", "a4")); got != line {
		t.Fatalf("a nameless party member's line was refused with %q, want %q", got, line)
	}
	if got := f.refusedUnmoved(t, lera, whisperCmd("nameless-whisper", "psst", "a4")); got != whisper {
		t.Fatalf("a nameless party member's whisper was refused with %q, want %q", got, whisper)
	}
	accepted(t, lera, say("named-line", "a1"))
	accepted(t, lera, whisperCmd("named-whisper", "psst", "a1"))

	p := speaker("p-1", identity.RolePlayer)
	for _, cmd := range []*vttv1.ClientCommand{sayCmd("a4", "", false), whisperCmd("", "psst", "a4")} {
		if err := gateway.Authorize(p, cmd, speakingWorld()); !errors.Is(err, gateway.ErrUnauthorized) {
			t.Fatalf("%v: err = %v, want one that wraps ErrUnauthorized", cmd.GetCommand(), err)
		}
	}
}
