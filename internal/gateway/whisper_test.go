package gateway_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func whisperCmd(requestID, text, from string, to ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_AddWhisper{
		AddWhisper: &vttv1.AddWhisper{Text: text, FromActorId: from, ToActorIds: to}}}
}

// whisperTable adds Tobbe, Ivo's party member, beside the fixture's Hero, and
// gives Lera a creature and a party member with no name.
func (f *gwFixture) whisperTable(t *testing.T) {
	t.Helper()
	party, other := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	ivo := f.playerIDFor(t, f.otherPlayerToken)
	for _, a := range []struct {
		id, name, to string
		kind         vttv1.ActorKind
	}{
		{"a2", "Tobbe", ivo, party},
		{"a3", "Goblin", f.playerID, other},
		{"a4", "", f.playerID, party},
	} {
		mustAppend(t, f.campaign, "whisper-actor-"+a.id, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{ActorId: a.id, Name: a.name}}})
		mustAppend(t, f.campaign, "whisper-grant-"+a.id, &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{ActorId: a.id, ParticipantId: a.to, Kind: a.kind}})
	}
}

func whispersOf(stream []*vttv1.Envelope) []*vttv1.WhisperAdded {
	var out []*vttv1.WhisperAdded
	for _, e := range stream {
		if w := e.GetWhisperAdded(); w != nil {
			out = append(out, w)
		}
	}
	return out
}

func narrated(stream []*vttv1.Envelope, text string) bool {
	return slices.ContainsFunc(stream, func(e *vttv1.Envelope) bool { return e.GetNarrationAdded().GetText() == text })
}

func (f *gwFixture) refusedUnmoved(t *testing.T, conn *websocket.Conn, cmd *vttv1.ClientCommand) string {
	t.Helper()
	before := f.head(t)
	sendCommand(t, conn, cmd)
	r := readResult(t, conn)
	if r.GetOk() {
		t.Fatalf("%s was accepted at sequence %d", cmd.GetRequestId(), r.GetSequence())
	}
	if after := f.head(t); after != before {
		t.Fatalf("%s was refused but the head moved from %d to %d", cmd.GetRequestId(), before, after)
	}
	return r.GetError()
}

func accepted(t *testing.T, conn *websocket.Conn, cmd *vttv1.ClientCommand) int64 {
	t.Helper()
	sendCommand(t, conn, cmd)
	r := readResult(t, conn)
	if !r.GetOk() || r.GetSequence() == 0 {
		t.Fatalf("%s: ok %v at sequence %d: %s", cmd.GetRequestId(), r.GetOk(), r.GetSequence(), r.GetError())
	}
	return r.GetSequence()
}

func narrateCmd(requestID, text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: text}}}
}

// VTT-582
func TestTheDMWhispersToAPartyMemberOverTheWire(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	ivo, lera := f.dial(f.otherPlayerToken, 0), f.dial(f.playerToken, 0)
	drainEvents(t, ivo, 300*time.Millisecond)
	drainEvents(t, lera, 300*time.Millisecond)
	dm := f.dial(f.dmToken, f.head(t))

	accepted(t, dm, whisperCmd("w-1", "an arrow from the dark", "", "a2"))
	world := readEvent(t, dm)
	if w := world.GetWhisperAdded(); w.GetText() != "an arrow from the dark" || !slices.Equal(w.GetToActorIds(), []string{"a2"}) ||
		world.GetParticipantId() != f.playerIDFor(t, f.dmToken) || world.GetActorRole() != "dm" {
		t.Fatalf("the world event = %v, want the DM's whisper to a2, naming the DM and the role", world)
	}
	accepted(t, dm, narrateCmd("n-1", "the wind turns"))

	toIvo, toLera := drainEvents(t, ivo, 500*time.Millisecond), drainEvents(t, lera, 500*time.Millisecond)
	if got := whispersOf(toIvo); len(got) != 1 || got[0].GetText() != "an arrow from the dark" || !narrated(toIvo, "the wind turns") {
		t.Fatalf("Tobbe's player was sent whispers %v and narration %v; want the whisper, then the narration", got, narrated(toIvo, "the wind turns"))
	}
	if got := whispersOf(toLera); len(got) != 0 || !narrated(toLera, "the wind turns") {
		t.Fatalf("Lera's player was sent whispers %v; want none, and the narration after", got)
	}
}

// VTT-587 VTT-162
func TestADMsWhisperToNoPartyMemberIsRefused(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	for _, token := range []string{f.dmToken, f.agentToken} {
		conn := f.dial(token, f.head(t))
		for _, cmd := range []*vttv1.ClientCommand{
			whisperCmd("to-nobody", "psst", ""),
			whisperCmd("to-a-creature", "psst", "", "a3"),
			whisperCmd("to-an-unknown-id", "psst", "", "ghost"),
			whisperCmd("from-an-actor", "psst", "a1", "a2"),
		} {
			f.refusedUnmoved(t, conn, cmd)
		}
		accepted(t, conn, whisperCmd("to-tobbe", "psst", "", "a2"))
	}
}

// VTT-583 VTT-588 VTT-589 VTT-590 VTT-162
func TestAPlayerWhispersToTheDMFromTheirOwnPartyMemberAlone(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	lera, ivo := f.dial(f.playerToken, 0), f.dial(f.otherPlayerToken, 0)
	drainEvents(t, lera, 300*time.Millisecond)
	drainEvents(t, ivo, 300*time.Millisecond)
	dm := f.dial(f.dmToken, f.head(t))

	accepted(t, lera, whisperCmd("from-hero", "I saw it", "a1"))
	if w := readEvent(t, dm).GetWhisperAdded(); w.GetFromActorId() != "a1" || len(w.GetToActorIds()) != 0 {
		t.Fatalf("the world holds %v, want Hero's whisper to the DM", w)
	}
	accepted(t, dm, narrateCmd("n-1", "the wind turns"))
	toLera, toIvo := drainEvents(t, lera, 500*time.Millisecond), drainEvents(t, ivo, 500*time.Millisecond)
	if got := whispersOf(toLera); len(got) != 1 || got[0].GetFromActorId() != "a1" || !narrated(toLera, "the wind turns") {
		t.Fatalf("Hero's log was sent whispers %v; want Hero's own, then the narration", got)
	}
	if got := whispersOf(toIvo); len(got) != 0 || !narrated(toIvo, "the wind turns") {
		t.Fatalf("Tobbe's log was sent whispers %v; want none, and the narration after", got)
	}

	var texts []string
	for _, from := range []string{"", "a2", "a3", "ghost"} {
		texts = append(texts, f.refusedUnmoved(t, lera, whisperCmd("from-"+from, "psst", from)))
	}
	for _, text := range texts[1:] {
		if text != texts[0] {
			t.Fatalf("refusals differ: %q", texts)
		}
	}
	f.refusedUnmoved(t, lera, whisperCmd("from-hero-to-tobbe", "psst", "a1", "a2"))
	if text := f.refusedUnmoved(t, lera, whisperCmd("from-nameless", "psst", "a4")); !strings.Contains(text, "no name") {
		t.Fatalf("a nameless party member's whisper was refused with %q, want it to say it has no name", text)
	}
}

// VTT-590
func TestASpectatorCannotWhisper(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	conn := f.dial(f.spectatorToken, f.head(t))
	f.refusedUnmoved(t, conn, whisperCmd("to-tobbe", "psst", "", "a2"))
	f.refusedUnmoved(t, conn, whisperCmd("from-hero", "psst", "a1"))
}
