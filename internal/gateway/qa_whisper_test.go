package gateway_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/coder/websocket"
)

func qaWhCmd(req string, w *vttv1.AddWhisper) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_AddWhisper{AddWhisper: w}}
}

// qaWhActor adds an actor and, when controller is set, grants it with kind.
func qaWhActor(t *testing.T, f *gwFixture, id, name string, kind vttv1.ActorKind, controller string) {
	t.Helper()
	mustAppend(t, f.campaign, "qa-wh-add-"+id, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: name},
	}})
	if controller != "" {
		mustAppend(t, f.campaign, "qa-wh-grant-"+id+"-"+controller, &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{ActorId: id, ParticipantId: controller, Kind: kind},
		})
	}
}

func qaWhSend(t *testing.T, conn *websocket.Conn, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	sendCommand(t, conn, cmd)
	return readResult(t, conn)
}

// qaWhRefused sends cmd and requires a refusal that appends nothing.
func qaWhRefused(t *testing.T, f *gwFixture, conn *websocket.Conn, cmd *vttv1.ClientCommand) string {
	t.Helper()
	before := f.head(t)
	res := qaWhSend(t, conn, cmd)
	if res.GetOk() {
		t.Fatalf("%s: accepted (sequence %d), want refused", cmd.GetRequestId(), res.GetSequence())
	}
	if res.GetError() == "" {
		t.Errorf("%s: refused with no error text", cmd.GetRequestId())
	}
	if after := f.head(t); after != before {
		t.Errorf("%s: head moved %d -> %d on a refusal", cmd.GetRequestId(), before, after)
	}
	return res.GetError()
}

// qaWhAccepted sends cmd, requires acceptance, and returns the event appended.
func qaWhAccepted(t *testing.T, f *gwFixture, conn *websocket.Conn, cmd *vttv1.ClientCommand) *vttv1.WhisperAdded {
	t.Helper()
	before := f.head(t)
	res := qaWhSend(t, conn, cmd)
	if !res.GetOk() {
		t.Fatalf("%s: refused %q, want accepted", cmd.GetRequestId(), res.GetError())
	}
	if after := f.head(t); after != before+1 {
		t.Fatalf("%s: head %d -> %d, want exactly one event", cmd.GetRequestId(), before, after)
	}
	events, unsub, _, err := f.campaign.Subscribe(before, 16)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()
	select {
	case env := <-events:
		w := env.GetWhisperAdded()
		if w == nil {
			t.Fatalf("%s: appended %T, want WhisperAdded", cmd.GetRequestId(), env.GetPayload())
		}
		if env.GetSequence() != res.GetSequence() {
			t.Errorf("result sequence %d, event sequence %d", res.GetSequence(), env.GetSequence())
		}
		return w
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: no event in the log after an accepted whisper", cmd.GetRequestId())
	}
	return nil
}

func qaWhSameIDs(got, want []string) bool {
	return strings.Join(got, "\x00") == strings.Join(want, "\x00") && len(got) == len(want)
}

// VTT-590, VTT-503
func TestQAWhTheDMWhispersToTwoPartyMembersAndTheEventCarriesEveryField(t *testing.T) {
	f := newGWFixture(t)
	qaWhActor(t, f, "qa-wh-b", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, f.playerID)
	dm := f.dial(f.dmToken, f.head(t))
	w := qaWhAccepted(t, f, dm, qaWhCmd("dm-1", &vttv1.AddWhisper{Text: "An arrow from the east.", ToActorIds: []string{"qa-wh-b", "a1"}}))
	if w.GetText() != "An arrow from the east." || !qaWhSameIDs(w.GetToActorIds(), []string{"qa-wh-b", "a1"}) || w.GetFromActorId() != "" {
		t.Errorf("event = %v, want the command's text and to_actor_ids in order, no from", w)
	}
}

// VTT-590
func TestQAWhTheAgentWhispersToAPartyMember(t *testing.T) {
	f := newGWFixture(t)
	agent := f.dial(f.agentToken, f.head(t))
	w := qaWhAccepted(t, f, agent, qaWhCmd("ag-1", &vttv1.AddWhisper{Text: "psst", ToActorIds: []string{"a1"}}))
	if !qaWhSameIDs(w.GetToActorIds(), []string{"a1"}) || w.GetText() != "psst" {
		t.Errorf("event = %v", w)
	}
}

// VTT-587, VTT-162
func TestQAWhADMOrAgentWhisperNamingNoPartyMemberIsRefusedAndAppendsNothing(t *testing.T) {
	f := newGWFixture(t)
	qaWhActor(t, f, "qa-wh-gob", "Goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, f.playerID)
	qaWhActor(t, f, "qa-wh-npc", "Barkeep", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, "")
	cases := map[string]*vttv1.AddWhisper{
		"no-to":              {Text: "x"},
		"to-creature":        {Text: "x", ToActorIds: []string{"qa-wh-gob"}},
		"to-ungranted-actor": {Text: "x", ToActorIds: []string{"qa-wh-npc"}},
		"to-unknown":         {Text: "x", ToActorIds: []string{"nobody-at-all"}},
		"to-party-and-gob":   {Text: "x", ToActorIds: []string{"a1", "qa-wh-gob"}},
		"to-empty-id":        {Text: "x", ToActorIds: []string{""}},
		"to-and-from":        {Text: "x", ToActorIds: []string{"a1"}, FromActorId: "a1"},
		"from-only":          {Text: "x", FromActorId: "a1"},
	}
	for _, tok := range []string{f.dmToken, f.agentToken} {
		conn := f.dial(tok, f.head(t))
		for name, w := range cases {
			qaWhRefused(t, f, conn, qaWhCmd(name, w))
		}
		qaWhAccepted(t, f, conn, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}}))
	}
}

// VTT-588, VTT-590, VTT-503
func TestQAWhAPlayerWhispersToTheDMFromTheirOwnPartyMember(t *testing.T) {
	f := newGWFixture(t)
	lera := f.dial(f.playerToken, f.head(t))
	w := qaWhAccepted(t, f, lera, qaWhCmd("pl-1", &vttv1.AddWhisper{Text: "I pocket the ring.", FromActorId: "a1"}))
	if w.GetFromActorId() != "a1" || len(w.GetToActorIds()) != 0 || w.GetText() != "I pocket the ring." {
		t.Errorf("event = %v, want from a1, to nobody, the text", w)
	}
}

// VTT-588, VTT-589, VTT-162
func TestQAWhAPlayersWhisperFromAnythingButTheirOwnPartyMemberIsRefusedWithOneText(t *testing.T) {
	f := newGWFixture(t)
	ivo := f.playerIDFor(t, f.otherPlayerToken)
	qaWhActor(t, f, "qa-wh-ivos", "Ivos Hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, ivo)
	qaWhActor(t, f, "qa-wh-pet", "Wolf", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, f.playerID)
	lera := f.dial(f.playerToken, f.head(t))
	texts := map[string]string{}
	for name, from := range map[string]string{"empty": "", "unknown": "no-such-actor", "not-theirs": "qa-wh-ivos", "creature": "qa-wh-pet"} {
		texts[name] = qaWhRefused(t, f, lera, qaWhCmd(name, &vttv1.AddWhisper{Text: "x", FromActorId: from}))
	}
	for name, text := range texts {
		if text != texts["empty"] {
			t.Errorf("%s refused with %q, empty from refused with %q: want one text", name, text, texts["empty"])
		}
	}
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
}

// VTT-588, VTT-162
func TestQAWhAPlayerCannotWhisperToACharacter(t *testing.T) {
	f := newGWFixture(t)
	ivo := f.playerIDFor(t, f.otherPlayerToken)
	qaWhActor(t, f, "qa-wh-ivos", "Ivos Hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, ivo)
	lera := f.dial(f.playerToken, f.head(t))
	qaWhRefused(t, f, lera, qaWhCmd("to-other", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"qa-wh-ivos"}, FromActorId: "a1"}))
	qaWhRefused(t, f, lera, qaWhCmd("to-own", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}, FromActorId: "a1"}))
	qaWhRefused(t, f, lera, qaWhCmd("to-only", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"qa-wh-ivos"}}))
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
}

// VTT-588, VTT-162
func TestQAWhANamelessPartyMemberCannotWhisperToTheDM(t *testing.T) {
	f := newGWFixture(t)
	qaWhActor(t, f, "qa-wh-anon", "", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, f.playerID)
	lera := f.dial(f.playerToken, f.head(t))
	qaWhRefused(t, f, lera, qaWhCmd("nameless", &vttv1.AddWhisper{Text: "x", FromActorId: "qa-wh-anon"}))
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
}

// VTT-588, VTT-162
func TestQAWhAPlayerWhoControlsNothingCannotWhisper(t *testing.T) {
	f := newGWFixture(t)
	ivo := f.dial(f.otherPlayerToken, f.head(t))
	qaWhRefused(t, f, ivo, qaWhCmd("ivo", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
	lera := f.dial(f.playerToken, f.head(t))
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
}

// VTT-590, VTT-162
func TestQAWhASpectatorCannotWhisperAtAll(t *testing.T) {
	f := newGWFixture(t)
	spec := f.dial(f.spectatorToken, f.head(t))
	qaWhRefused(t, f, spec, qaWhCmd("sp-to", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}}))
	qaWhRefused(t, f, spec, qaWhCmd("sp-from", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"}))
	dm := f.dial(f.dmToken, f.head(t))
	qaWhAccepted(t, f, dm, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}}))
}

// VTT-590
func TestQAWhAuthorizeAdmitsAWhisperFromTheDMTheAgentAndAPlayerAlone(t *testing.T) {
	f := newGWFixture(t)
	st := f.campaign.State()
	toA1 := qaWhCmd("r", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}})
	fromA1 := qaWhCmd("r", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"})
	cases := []struct {
		role identity.Role
		cmd  *vttv1.ClientCommand
		ok   bool
	}{
		{identity.RoleDM, toA1, true},
		{identity.RoleAgent, toA1, true},
		{identity.RolePlayer, fromA1, true},
		{identity.RoleSpectator, toA1, false},
		{identity.RoleSpectator, fromA1, false},
		{identity.Role("gm"), toA1, false},
		{identity.Role(""), fromA1, false},
	}
	for _, c := range cases {
		p := &identity.Participant{ID: f.playerID, Name: "Lera", Role: c.role}
		err := gateway.Authorize(p, c.cmd, st)
		if c.ok && err != nil {
			t.Errorf("role %q: refused %v, want admitted", c.role, err)
		}
		if !c.ok && !errors.Is(err, gateway.ErrUnauthorized) {
			t.Errorf("role %q: got %v, want ErrUnauthorized", c.role, err)
		}
	}
}

// VTT-503
func TestQAWhToEventCarriesEveryFieldOfTheCommand(t *testing.T) {
	p := &identity.Participant{ID: "p-dm", Name: "Dee", Role: identity.RoleDM}
	env, err := gateway.ToEvent(qaWhCmd("r", &vttv1.AddWhisper{Text: "hi", ToActorIds: []string{"z", "a"}, FromActorId: "f"}), p)
	if err != nil {
		t.Fatalf("ToEvent: %v", err)
	}
	w := env.GetWhisperAdded()
	if w == nil || w.GetText() != "hi" || !qaWhSameIDs(w.GetToActorIds(), []string{"z", "a"}) || w.GetFromActorId() != "f" {
		t.Errorf("ToEvent = %v, want every field as it came", env)
	}
	if env.GetParticipantId() != "p-dm" || env.GetActorRole() != "dm" || env.GetEventId() == "" {
		t.Errorf("stamps = %q %q %q", env.GetParticipantId(), env.GetActorRole(), env.GetEventId())
	}
}

// VTT-585, VTT-162
func TestQAWhAWhispersTextIsBoundedOverTheWire(t *testing.T) {
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, f.head(t))
	qaWhRefused(t, f, dm, qaWhCmd("empty", &vttv1.AddWhisper{Text: "", ToActorIds: []string{"a1"}}))
	qaWhRefused(t, f, dm, qaWhCmd("long", &vttv1.AddWhisper{Text: strings.Repeat("é", 4096) + "x", ToActorIds: []string{"a1"}}))
	qaWhAccepted(t, f, dm, qaWhCmd("cap", &vttv1.AddWhisper{Text: strings.Repeat("é", 4096), ToActorIds: []string{"a1"}}))
}

// VTT-587, VTT-588, VTT-589
func TestQAWhAPartyMemberRegrantedAsACreatureNeitherHearsNorWhispers(t *testing.T) {
	f := newGWFixture(t)
	mustAppend(t, f.campaign, "qa-wh-regrant", &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
	})
	qaWhActor(t, f, "qa-wh-b", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, f.playerID)
	dm := f.dial(f.dmToken, f.head(t))
	qaWhRefused(t, f, dm, qaWhCmd("dm-to-creature", &vttv1.AddWhisper{Text: "x", ToActorIds: []string{"a1"}}))
	lera := f.dial(f.playerToken, f.head(t))
	empty := qaWhRefused(t, f, lera, qaWhCmd("empty", &vttv1.AddWhisper{Text: "x"}))
	if got := qaWhRefused(t, f, lera, qaWhCmd("from-creature", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"})); got != empty {
		t.Errorf("from a regranted creature refused with %q, want the one text %q", got, empty)
	}
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "qa-wh-b"}))
}

// VTT-588, VTT-589
func TestQAWhAPlayerWhoseControlWasRevokedCannotWhisperFromIt(t *testing.T) {
	f := newGWFixture(t)
	qaWhActor(t, f, "qa-wh-b", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, f.playerID)
	if _, err := f.campaign.Append(&vttv1.Envelope{EventId: "qa-wh-revoke", Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID},
	}}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	lera := f.dial(f.playerToken, f.head(t))
	empty := qaWhRefused(t, f, lera, qaWhCmd("empty", &vttv1.AddWhisper{Text: "x"}))
	if got := qaWhRefused(t, f, lera, qaWhCmd("revoked", &vttv1.AddWhisper{Text: "x", FromActorId: "a1"})); got != empty {
		t.Errorf("from a revoked party member refused with %q, want the one text %q", got, empty)
	}
	qaWhAccepted(t, f, lera, qaWhCmd("control", &vttv1.AddWhisper{Text: "x", FromActorId: "qa-wh-b"}))
}
