package campaign_test

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// whisperTable is a hall where tobbe and lera stand in the two rooms and brann
// stands nowhere.
func whisperTable(t *testing.T) *campaign.Campaign {
	t.Helper()
	c := openTemp(t)
	must(t, c, cenv(nextID(), twoRooms()))
	for _, id := range []string{"tobbe", "lera", "brann"} {
		must(t, c, cenv(nextID(), partyMember(id)))
	}
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-tobbe", SceneId: "s", ActorId: "tobbe", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-lera", SceneId: "s", ActorId: "lera", Position: at(5, 1)}))
	return c
}

func whispersIn(entries []campaign.CharacterEntry) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, e := range entries {
		if e.Env.GetWhisperAdded() != nil {
			out = append(out, e.Env)
		}
	}
	return out
}

// asWritten is the world's whisper as it is written into a log: no sequence,
// no issuer, and only that log's own character among the recipients.
func asWritten(world *vttv1.Envelope, own string) *vttv1.Envelope {
	c := proto.Clone(world).(*vttv1.Envelope)
	c.Sequence, c.ParticipantId, c.ActorRole = 0, "", ""
	w := c.GetWhisperAdded()
	if slices.Contains(w.ToActorIds, own) {
		w.ToActorIds = []string{own}
	}
	return c
}

func sameWhisper(t *testing.T, who string, got, world *vttv1.Envelope) {
	t.Helper()
	g := proto.Clone(got).(*vttv1.Envelope)
	g.Sequence = 0
	if want := asWritten(world, who); !proto.Equal(g, want) {
		t.Errorf("%s's log holds\n %v\nwant\n %v", who, got, want)
	}
}

func whispered(id, role, participant string, w *vttv1.WhisperAdded) *vttv1.Envelope {
	e := cenv(id, w)
	e.ActorRole, e.ParticipantId = role, participant
	return e
}

// VTT-582 VTT-584 VTT-272
func TestAWhisperReachesTheWorldAndTheLogsItNamesAlone(t *testing.T) {
	c := whisperTable(t)
	must(t, c, whispered(nextID(), "dm", "p-dm", &vttv1.WhisperAdded{
		Text: "an arrow from the dark", ToActorIds: []string{"tobbe"}}))
	must(t, c, whispered(nextID(), "dm", "p-dm", &vttv1.WhisperAdded{
		Text: "you two hear it", ToActorIds: []string{"brann", "tobbe"}}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the wind turns"}))
	must(t, c, cenv(nextID(), partyMember("late")))

	var world []*vttv1.Envelope
	for _, e := range drainLog(t, c, 10) {
		if e.GetWhisperAdded() != nil {
			world = append(world, e)
		}
	}
	if len(world) != 2 {
		t.Fatalf("the world log holds %d whispers, want 2", len(world))
	}

	tobbe := whispersIn(logOf(t, c, "tobbe"))
	if len(tobbe) != 2 {
		t.Fatalf("tobbe's log holds %d whispers, want 2: %v", len(tobbe), kindsOf(logOf(t, c, "tobbe")))
	}
	sameWhisper(t, "tobbe", tobbe[0], world[0])
	sameWhisper(t, "tobbe", tobbe[1], world[1])

	brann := whispersIn(logOf(t, c, "brann"))
	if len(brann) != 1 {
		t.Fatalf("brann's log holds %d whispers, want 1", len(brann))
	}
	sameWhisper(t, "brann", brann[0], world[1])

	for _, id := range []string{"lera", "late"} {
		if got := whispersIn(logOf(t, c, id)); len(got) != 0 {
			t.Errorf("%s's log holds a whisper it was not addressed: %v", id, got)
		}
	}
	for _, id := range []string{"tobbe", "lera", "brann"} {
		if !hasKind(logOf(t, c, id), "NarrationAdded the wind turns") {
			t.Errorf("%s's log lacks the narration every log is told", id)
		}
	}
	for _, id := range []string{"tobbe", "lera", "brann", "late"} {
		assertFoldsAtEveryPrefix(t, id, logOf(t, c, id))
	}
}

// VTT-583
func TestAPartyMembersWhisperReachesTheWorldAndItsOwnLogAlone(t *testing.T) {
	c := whisperTable(t)
	must(t, c, whispered(nextID(), "player", "p-kim", &vttv1.WhisperAdded{
		Text: "I saw it", FromActorId: "tobbe"}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the wind turns"}))

	var world []*vttv1.Envelope
	for _, e := range drainLog(t, c, 8) {
		if e.GetWhisperAdded() != nil {
			world = append(world, e)
		}
	}
	if len(world) != 1 {
		t.Fatalf("the world log holds %d whispers, want 1", len(world))
	}
	tobbe := whispersIn(logOf(t, c, "tobbe"))
	if len(tobbe) != 1 {
		t.Fatalf("tobbe's log holds %d whispers, want 1", len(tobbe))
	}
	sameWhisper(t, "tobbe", tobbe[0], world[0])
	for _, id := range []string{"lera", "brann"} {
		if got := whispersIn(logOf(t, c, id)); len(got) != 0 {
			t.Errorf("%s's log holds tobbe's whisper to the DM: %v", id, got)
		}
		if !hasKind(logOf(t, c, id), "NarrationAdded the wind turns") {
			t.Errorf("%s's log lacks the narration every log is told", id)
		}
	}
}
