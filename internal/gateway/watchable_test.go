package gateway_test

import (
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// anyFrame returns the next frame that is not presence, a WatchableChanged
// included, or nil after d or once the connection closed.
func (w *watchConn) anyFrame(d time.Duration) *vttv1.ServerFrame {
	w.t.Helper()
	for {
		var fr *vttv1.ServerFrame
		if len(w.pending) > 0 {
			fr, w.pending = w.pending[0], w.pending[1:]
		} else {
			select {
			case fr = <-w.frames:
			case <-w.closed:
				return nil
			case <-time.After(d):
				return nil
			}
		}
		if fr.GetPresenceSnapshot() == nil && fr.GetPresenceChanged() == nil {
			return fr
		}
	}
}

func (w *watchConn) mustAny() *vttv1.ServerFrame {
	w.t.Helper()
	fr := w.anyFrame(3 * time.Second)
	if fr == nil {
		w.t.Fatal("no frame within 3s")
	}
	return fr
}

// setOf renders a set as "id:name" pairs, failing on any other frame.
func setOf(t *testing.T, what string, fr *vttv1.ServerFrame) string {
	t.Helper()
	set := fr.GetWatchableChanged()
	if set == nil {
		t.Fatalf("%s: got %v, want a WatchableChanged", what, fr)
	}
	pairs := make([]string, len(set.GetActors()))
	for i, a := range set.GetActors() {
		pairs[i] = a.GetActorId() + ":" + a.GetName()
	}
	return strings.Join(pairs, " ")
}

func requireSet(t *testing.T, what string, fr *vttv1.ServerFrame, want string) {
	t.Helper()
	if got := setOf(t, what, fr); got != want {
		t.Fatalf("%s: the set is [%s], want [%s]", what, got, want)
	}
}

func requireViewpoint(t *testing.T, what string, fr *vttv1.ServerFrame, want string) {
	t.Helper()
	vc := fr.GetViewpointChanged()
	if vc == nil {
		t.Fatalf("%s: got %v, want ViewpointChanged %q", what, fr, want)
	}
	if vc.GetActorId() != want {
		t.Fatalf("%s: ViewpointChanged names %q, want %q", what, vc.GetActorId(), want)
	}
}

// requireEntries reads the entries actorID's log holds after its first skip,
// failing on any other frame between them.
func (w *watchConn) requireEntries(f *gwFixture, what, actorID string, skip int) {
	w.t.Helper()
	want := logOf(w.t, f, actorID)[skip:]
	got := make([]*vttv1.Envelope, 0, len(want))
	for len(got) < len(want) {
		fr := w.mustAny()
		if fr.GetEvent() == nil {
			w.t.Fatalf("%s: entry %d of %d is %v", what, len(got)+1, len(want), fr)
		}
		got = append(got, fr.GetEvent())
	}
	requireLog(w.t, what, got, want)
}

func (w *watchConn) quietAll(d time.Duration) {
	w.t.Helper()
	if fr := w.anyFrame(d); fr != nil {
		w.t.Fatalf("want nothing within %s, got %v", d, fr)
	}
	select {
	case <-w.closed:
		w.t.Fatal("the connection closed, so its silence shows nothing")
	default:
	}
}

// openWithSet reads the head, then the set, then the ViewpointChanged
// (VTT-368, VTT-399).
func (w *watchConn) openWithSet(set, viewpoint string) {
	w.t.Helper()
	if head := w.mustAny(); head.GetCatchUpHead() == nil {
		w.t.Fatalf("the first frame is %v, want the catch-up head", head)
	}
	requireSet(w.t, "the frame after the head", w.mustAny(), set)
	requireViewpoint(w.t, "the frame after the set", w.mustAny(), viewpoint)
}

// VTT-399
func TestAConnectionIsToldWhomItMayWatchBeforeItsOpeningViewpoint(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "p2", Name: "Quill",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Goblin",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})

	player := f.watch(f.playerToken, 0, "")
	player.openWithSet("a1:Hero", "a1")
	player.requireEntries(f, "the player's log after its opening", "a1", 0)

	spectator := f.watch(f.spectatorToken, 0, "")
	spectator.openWithSet("a1:Hero p2:Quill", "")

	other := f.watch(f.otherPlayerToken, 0, "")
	other.openWithSet("a1:Hero p2:Quill", "")

	player.quietAll(300 * time.Millisecond)
	spectator.quietAll(100 * time.Millisecond)
	other.quietAll(100 * time.Millisecond)
}

// VTT-400
func TestTheSetIsSentAgainWhenItChanges(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.playerToken, 0, "")
	w.openWithSet("a1:Hero", "a1")
	w.requireEntries(f, "a1's log", "a1", 0)

	held := len(logOf(t, f, "a1"))
	appendEvents(t, f.campaign, partyMemberOf("a2", f.playerID)...)
	requireSet(t, "after a grant of a party member a1 does not see", w.mustAny(), "a1:Hero a2:a2")
	w.requireEntries(f, "a1's log after the grant", "a1", held)
	w.quietAll(300 * time.Millisecond)

	held = len(logOf(t, f, "a1"))
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "nothing changes whom anyone may watch"})
	w.requireEntries(f, "a1's log after a narration", "a1", held)
	w.quietAll(300 * time.Millisecond)

	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID})
	requireSet(t, "after the revocation", w.mustAny(), "a2:a2")
	requireViewpoint(t, "after the revocation's set", w.mustAny(), "a2")
	w.requireEntries(f, "a2's log from its first entry", "a2", 0)

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: f.playerID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	requireSet(t, "after a2 left the party", w.mustAny(), "a1:Hero")
	requireViewpoint(t, "after that set", w.mustAny(), "")

	appendEvents(t, f.campaign, partyMemberOf("a3", f.playerID)...)
	requireSet(t, "after a grant to a player watching nobody", w.mustAny(), "a3:a3")
	requireViewpoint(t, "after a3's set", w.mustAny(), "a3")
	w.requireEntries(f, "a3's log from its first entry", "a3", 0)

	appendEvents(t, f.campaign, &vttv1.ActorRemoved{ActorId: "a3"})
	requireSet(t, "after the removal", w.mustAny(), "a1:Hero")
	requireViewpoint(t, "after the removal's set", w.mustAny(), "")
	w.quietAll(300 * time.Millisecond)
}

// VTT-400, VTT-386
func TestASpectatorIsToldOfEveryPartyMember(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.spectatorToken, 0, "")
	w.openWithSet("a1:Hero", "")

	appendEvents(t, f.campaign, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "c1", Name: "Corin",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	requireSet(t, "after a party member was added", w.mustAny(), "a1:Hero c1:Corin")
	w.quietAll(300 * time.Millisecond)

	dmID := participantID(t, f, f.dmToken)
	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "c1", ParticipantId: dmID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	requireSet(t, "after c1 left the party", w.mustAny(), "a1:Hero")
	w.quietAll(300 * time.Millisecond)

	if r := w.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("perching on a1 was refused: %s", r.GetError())
	}
	requireViewpoint(t, "a perch, which changes no set", w.mustAny(), "a1")
	w.requireEntries(f, "a1's log from its first entry", "a1", 0)

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: f.playerID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	requireSet(t, "after the watched shoulder left the party", w.mustAny(), "")
	requireViewpoint(t, "after the empty set", w.mustAny(), "")
	w.quietAll(300 * time.Millisecond)
}

// VTT-400
func TestAPromotionTellsTheSeatTheNewRolesSet(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	id := participantID(t, f, f.spectatorToken)
	appendEvents(t, f.campaign, partyMemberOf("c1", id)...)
	w := f.watch(f.spectatorToken, 0, "a1")
	w.openWithSet("a1:Hero c1:c1", "a1")
	w.requireEntries(f, "a1's log", "a1", 0)

	promote(t, dm, id, identity.RolePlayer)
	requireSet(t, "after the promotion to a player who controls c1", w.mustAny(), "c1:c1")
	requireViewpoint(t, "after the promoted role's set", w.mustAny(), "c1")
	w.requireEntries(f, "c1's log from its first entry", "c1", 0)
	w.quietAll(300 * time.Millisecond)
}

// VTT-401
func TestTheDMAndTheAgentAreToldNoSet(t *testing.T) {
	f := newGWFixture(t)
	dm, agent := f.watch(f.dmToken, 0, ""), f.watch(f.agentToken, 0, "")
	player := f.watch(f.playerToken, 0, "")
	player.openWithSet("a1:Hero", "a1")
	player.requireEntries(f, "a1's log", "a1", 0)

	grant := appendEvents(t, f.campaign, partyMemberOf("a2", f.playerID)...)
	requireSet(t, "the player, after the grant", player.mustAny(), "a1:Hero a2:a2")
	for who, w := range map[string]*watchConn{"the DM": dm, "the agent": agent} {
		sawGrant := false
		for fr := w.anyFrame(300 * time.Millisecond); fr != nil; fr = w.anyFrame(300 * time.Millisecond) {
			if fr.GetWatchableChanged() != nil || fr.GetViewpointChanged() != nil {
				t.Fatalf("%s was sent %v", who, fr)
			}
			sawGrant = sawGrant || fr.GetEvent().GetEventId() == grant[1].GetEventId()
		}
		if !sawGrant {
			t.Fatalf("%s was not sent the grant, so its silence shows nothing", who)
		}
	}
}

// VTT-452 VTT-339 VTT-342 VTT-399
func TestAPlayerWhoControlsNoPartyMemberPerchesAndReopensOnIt(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "p2", Name: "Quill",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})

	ivo := f.watch(f.otherPlayerToken, 0, "")
	ivo.openWithSet("a1:Hero p2:Quill", "")
	if r := ivo.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("a player who controls no party member was refused a1: %s", r.GetError())
	}
	requireViewpoint(t, "after the perch", ivo.mustAny(), "a1")
	ivo.requireEntries(f, "a1's log after the perch", "a1", 0)

	again := f.watch(f.otherPlayerToken, 0, "a1")
	again.openWithSet("a1:Hero p2:Quill", "a1")
	again.requireEntries(f, "a1's log on a dial naming it", "a1", 0)
	ivo.quietAll(300 * time.Millisecond)
}

// VTT-340 VTT-400 VTT-452
func TestAPerchedPlayerGrantedAPartyMemberMovesToTheFirst(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "p2", Name: "Quill",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})

	ivo := f.watch(f.otherPlayerToken, 0, "")
	ivo.openWithSet("a1:Hero p2:Quill", "")
	if r := ivo.command(setViewpoint("p2")); !r.GetOk() {
		t.Fatalf("a player who controls no party member was refused p2: %s", r.GetError())
	}
	requireViewpoint(t, "after the perch", ivo.mustAny(), "p2")
	ivo.requireEntries(f, "p2's log after the perch", "p2", 0)

	ivoID := participantID(t, f, f.otherPlayerToken)
	appendEvents(t, f.campaign, append(partyMemberOf("x9", ivoID), partyMemberOf("x8", ivoID)...)...)
	requireSet(t, "after the grants", ivo.mustAny(), "x8:x8 x9:x9")
	requireViewpoint(t, "after the set", ivo.mustAny(), "x8")
	ivo.requireEntries(f, "x8's log after the switch", "x8", 0)
	ivo.quietAll(300 * time.Millisecond)
}

// VTT-454
func TestTheSetSaysWhetherTheParticipantControlsNoPartyMember(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "pet", Name: "Pet", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "pet", ParticipantId: participantID(t, f, f.otherPlayerToken),
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	noneToken, _, err := f.ids.CreateInvite("Nell", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	coToken, coID, err := f.ids.CreateInvite("Mira", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: coID,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	for _, c := range []struct {
		who, token string
		want       bool
	}{
		{"a player who controls a1", f.playerToken, false},
		{"a player who controls only a non-party actor", f.otherPlayerToken, true},
		{"a player who controls nothing", noneToken, true},
		{"a spectator who controls nothing", f.spectatorToken, true},
		{"a player who holds a1 after another player", coToken, false},
	} {
		w := f.watch(c.token, 0, "")
		if head := w.mustAny(); head.GetCatchUpHead() == nil {
			t.Fatalf("%s: the first frame is %v, want the catch-up head", c.who, head)
		}
		set := w.mustAny().GetWatchableChanged()
		if set == nil || set.GetControlsNoPartyMember() != c.want {
			t.Errorf("%s: the opening set is %v, want controls_no_party_member %v", c.who, set, c.want)
		}
	}
}

// VTT-455 VTT-454
func TestARevocationOfATablesOnlyPartyMemberSendsTheSetAgain(t *testing.T) {
	f := newGWFixture(t)
	lera := f.watch(f.playerToken, 0, "")
	lera.openWithSet("a1:Hero", "a1")
	lera.requireEntries(f, "a1's log at the opening", "a1", 0)
	seen := len(logOf(t, f, "a1"))

	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID})
	set := lera.mustAny()
	requireSet(t, "after the revocation", set, "a1:Hero")
	if !set.GetWatchableChanged().GetControlsNoPartyMember() {
		t.Fatal("the set after the revocation does not say the player controls no party member")
	}
	lera.requireEntries(f, "the revocation's entry after the set", "a1", seen)
	lera.quietAll(300 * time.Millisecond)
}
