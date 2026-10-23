package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaWatchableWait  = 3 * time.Second
	qaWatchableQuiet = 400 * time.Millisecond
)

type qaWatchableNames map[string]string

func qaWatchableMember(actorID, name, participantID string) []proto.Message {
	return []proto.Message{
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: actorID, Name: name,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		&vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: participantID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}
}

func qaWatchableUncontrolled(actorID, name string) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: actorID, Name: name,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
}

// Keep the set: watchConn.next drops it. This skips presence alone.
func qaWatchableNext(w *watchConn, d time.Duration) *vttv1.ServerFrame {
	w.t.Helper()
	timeout := time.After(d)
	for {
		var fr *vttv1.ServerFrame
		if len(w.pending) > 0 {
			fr, w.pending = w.pending[0], w.pending[1:]
		} else {
			select {
			case fr = <-w.frames:
			case <-w.closed:
				return nil
			case <-timeout:
				return nil
			}
		}
		if fr.GetPresenceSnapshot() == nil && fr.GetPresenceChanged() == nil {
			return fr
		}
	}
}

func qaWatchableUntil(w *watchConn, what string, stop func(*vttv1.ServerFrame) bool) []*vttv1.ServerFrame {
	w.t.Helper()
	var out []*vttv1.ServerFrame
	deadline := time.Now().Add(qaWatchableWait)
	for {
		fr := qaWatchableNext(w, time.Until(deadline))
		if fr == nil {
			w.t.Fatalf("no %s within %s; read %v", what, qaWatchableWait, out)
		}
		out = append(out, fr)
		if stop(fr) {
			return out
		}
	}
}

func qaWatchableDrain(w *watchConn) []*vttv1.ServerFrame {
	w.t.Helper()
	var out []*vttv1.ServerFrame
	for {
		fr := qaWatchableNext(w, qaWatchableQuiet)
		if fr == nil {
			return out
		}
		out = append(out, fr)
	}
}

func qaWatchableIsSet(fr *vttv1.ServerFrame) bool { return fr.GetWatchableChanged() != nil }

func qaWatchableIsViewpoint(fr *vttv1.ServerFrame) bool { return fr.GetViewpointChanged() != nil }

func qaWatchableIndex(frames []*vttv1.ServerFrame, is func(*vttv1.ServerFrame) bool) int {
	for i, fr := range frames {
		if is(fr) {
			return i
		}
	}
	return -1
}

func qaWatchableCount(frames []*vttv1.ServerFrame, is func(*vttv1.ServerFrame) bool) int {
	n := 0
	for _, fr := range frames {
		if is(fr) {
			n++
		}
	}
	return n
}

// Warning: a nil set must fail, or an absent frame passes for an empty one.
func qaWatchableRequire(t *testing.T, who string, got *vttv1.WatchableChanged, want qaWatchableNames) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: no set, want %v", who, want)
	}
	names := qaWatchableNames{}
	for _, a := range got.GetActors() {
		if _, dup := names[a.GetActorId()]; dup {
			t.Fatalf("%s: the set names %q twice: %v", who, a.GetActorId(), got)
		}
		names[a.GetActorId()] = a.GetName()
	}
	if !maps.Equal(names, want) {
		t.Fatalf("%s: the set names %v, want %v", who, names, want)
	}
}

func qaWatchableOpening(w *watchConn) (*vttv1.WatchableChanged, string) {
	w.t.Helper()
	for i, want := range []string{"catch-up head", "presence snapshot"} {
		var fr *vttv1.ServerFrame
		select {
		case fr = <-w.frames:
		case <-time.After(qaWatchableWait):
			w.t.Fatalf("no frame %d within %s", i+1, qaWatchableWait)
		}
		if (i == 0 && fr.GetCatchUpHead() == nil) || (i == 1 && fr.GetPresenceSnapshot() == nil) {
			w.t.Fatalf("frame %d is %v, want the %s", i+1, fr, want)
		}
	}
	var set *vttv1.WatchableChanged
	for _, fr := range qaWatchableUntil(w, "opening ViewpointChanged", qaWatchableIsViewpoint) {
		switch {
		case fr.GetWatchableChanged() != nil:
			if set != nil {
				w.t.Fatalf("two sets before the opening ViewpointChanged: %v, %v", set, fr)
			}
			set = fr.GetWatchableChanged()
		case fr.GetViewpointChanged() != nil:
			if set == nil {
				w.t.Fatal("the opening ViewpointChanged came before any set")
			}
			return set, fr.GetViewpointChanged().GetActorId()
		default:
			w.t.Fatalf("%v came before the opening ViewpointChanged", fr)
		}
	}
	return nil, ""
}

func qaWatchableEntries(w *watchConn, n int) []*vttv1.Envelope {
	w.t.Helper()
	out := make([]*vttv1.Envelope, 0, n)
	for len(out) < n {
		fr := qaWatchableNext(w, qaWatchableWait)
		if fr.GetEvent() == nil {
			w.t.Fatalf("entry %d of %d: got %v", len(out)+1, n, fr)
		}
		out = append(out, fr.GetEvent())
	}
	return out
}

func qaWatchableOpenLog(t *testing.T, f *gwFixture, w *watchConn, wantVP string, want qaWatchableNames) {
	t.Helper()
	set, vp := qaWatchableOpening(w)
	if vp != wantVP {
		t.Fatalf("opens on %q, want %q", vp, wantVP)
	}
	qaWatchableRequire(t, "the opening set", set, want)
	if vp != "" {
		log := logOf(t, f, vp)
		requireLog(t, "the opening log", qaWatchableEntries(w, len(log)), log)
	}
}

func qaWatchableOneSet(t *testing.T, who string, w *watchConn) []*vttv1.ServerFrame {
	t.Helper()
	frames := qaWatchableUntil(w, who+"'s set", qaWatchableIsSet)
	frames = append(frames, qaWatchableDrain(w)...)
	if n := qaWatchableCount(frames, qaWatchableIsSet); n != 1 {
		t.Fatalf("%s: %d sets for one change, want one: %v", who, n, frames)
	}
	return frames
}

func qaWatchableSetOf(frames []*vttv1.ServerFrame) *vttv1.WatchableChanged {
	if i := qaWatchableIndex(frames, qaWatchableIsSet); i >= 0 {
		return frames[i].GetWatchableChanged()
	}
	return nil
}

func qaWatchableRequireChange(t *testing.T, who string, w *watchConn, want qaWatchableNames) {
	t.Helper()
	frames := qaWatchableOneSet(t, who, w)
	qaWatchableRequire(t, who, qaWatchableSetOf(frames), want)
	if i := qaWatchableIndex(frames, qaWatchableIsViewpoint); i >= 0 {
		t.Fatalf("%s: the change moved the seat to %q, want it to stay", who, frames[i].GetViewpointChanged().GetActorId())
	}
}

func qaWatchableRequireMove(t *testing.T, who string, w *watchConn, want qaWatchableNames, wantVP string) {
	t.Helper()
	frames := qaWatchableOneSet(t, who, w)
	qaWatchableRequire(t, who, qaWatchableSetOf(frames), want)
	s, v := qaWatchableIndex(frames, qaWatchableIsSet), qaWatchableIndex(frames, qaWatchableIsViewpoint)
	if v < 0 {
		t.Fatalf("%s: no ViewpointChanged after the change: %v", who, frames)
	}
	if got := frames[v].GetViewpointChanged().GetActorId(); got != wantVP {
		t.Fatalf("%s: moved to %q, want %q", who, got, wantVP)
	}
	if s > v {
		t.Fatalf("%s: the set came after the ViewpointChanged it caused: %v", who, frames)
	}
}

func qaWatchableRequireNoSet(t *testing.T, who string, frames []*vttv1.ServerFrame) {
	t.Helper()
	if i := qaWatchableIndex(frames, qaWatchableIsSet); i >= 0 {
		t.Fatalf("%s: sent a set when nothing changed it: %v", who, frames[i])
	}
}

// VTT-399; the set comes before the ViewpointChanged it causes (SPEC-015)
func TestQAWatchableAPlayersOpeningNamesTheirPartyMembersByName(t *testing.T) {
	f := newGWFixture(t)
	ivo := participantID(t, f, f.otherPlayerToken)
	appendEvents(t, f.campaign, qaWatchableMember("a0", "Zed", f.playerID)...)
	appendEvents(t, f.campaign, qaWatchableMember("b1", "Bryn", ivo)...)
	appendEvents(t, f.campaign, qaWatchableUncontrolled("pm", "Wanderer"))
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Gob", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "gob", ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})

	qaWatchableOpenLog(t, f, f.watch(f.playerToken, 0, ""), "a0", qaWatchableNames{"a0": "Zed", "a1": "Hero"})
	qaWatchableOpenLog(t, f, f.watch(f.playerToken, 0, "a1"), "a1", qaWatchableNames{"a0": "Zed", "a1": "Hero"})
	qaWatchableOpenLog(t, f, f.watch(f.otherPlayerToken, 0, ""), "b1", qaWatchableNames{"b1": "Bryn"})
}

// VTT-399
func TestQAWatchableASpectatorsOpeningNamesEveryPartyMember(t *testing.T) {
	f := newGWFixture(t)
	ivo := participantID(t, f, f.otherPlayerToken)
	appendEvents(t, f.campaign, qaWatchableMember("a0", "Zed", f.playerID)...)
	appendEvents(t, f.campaign, qaWatchableMember("b1", "Bryn", ivo)...)
	appendEvents(t, f.campaign, qaWatchableUncontrolled("pm", "Wanderer"))
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Gob", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "gob", ParticipantId: participantID(t, f, f.spectatorToken),
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	all := qaWatchableNames{"a0": "Zed", "a1": "Hero", "b1": "Bryn", "pm": "Wanderer"}

	qaWatchableOpenLog(t, f, f.watch(f.spectatorToken, 0, ""), "", all)
	qaWatchableOpenLog(t, f, f.watch(f.spectatorToken, 0, "b1"), "b1", all)
}

// VTT-399 VTT-454: "even when there are none"
func TestQAWatchableAConnectionWithNoneIsSentAnEmptySet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign,
		&vttv1.TokenRemoved{TokenId: "t1"}, &vttv1.ActorRemoved{ActorId: "a1"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wolf", Name: "Wolf", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "wolf", ParticipantId: participantID(t, f, f.otherPlayerToken),
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	qaWatchableOpenLog(t, f, f.watch(f.otherPlayerToken, 0, ""), "", qaWatchableNames{})

	conn := f.dialOn(f.otherPlayerToken, 0, "")
	raw := make(chan []byte, 64)
	go func() {
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				close(raw)
				return
			}
			raw <- b
		}
	}()
	deadline := time.After(qaWatchableWait)
	for found := false; !found; {
		select {
		case b, ok := <-raw:
			if !ok {
				t.Fatal("the connection closed before the set")
			}
			var arms map[string]json.RawMessage
			if err := json.Unmarshal(b, &arms); err != nil {
				t.Fatal(err)
			}
			body, isSet := arms["watchableChanged"]
			if !isSet {
				continue
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, body); err != nil {
				t.Fatal(err)
			}
			if compact.String() != `{"controlsNoPartyMember":true}` {
				t.Fatalf("the empty set arrives as %s, want only the flag", b)
			}
			found = true
		case <-deadline:
			t.Fatal("no set on the wire within 3s")
		}
	}
	conn.Close(websocket.StatusNormalClosure, "")

	f2 := newGWFixture(t)
	appendEvents(t, f2.campaign, &vttv1.TokenRemoved{TokenId: "t1"}, &vttv1.ActorRemoved{ActorId: "a1"})
	qaWatchableOpenLog(t, f2, f2.watch(f2.spectatorToken, 0, ""), "", qaWatchableNames{})
}

// VTT-400; the set comes before the ViewpointChanged it causes (SPEC-015)
func TestQAWatchableAGrantChangesTheSetsItReaches(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, qaWatchableUncontrolled("pm", "Wanderer"))
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero"})
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero", "pm": "Wanderer"})
	ivo := f.watch(f.otherPlayerToken, 0, "")
	qaWatchableOpenLog(t, f, ivo, "", qaWatchableNames{"a1": "Hero", "pm": "Wanderer"})

	appendEvents(t, f.campaign, qaWatchableMember("bd", "Bard", f.playerID)...)
	qaWatchableRequireChange(t, "the grantee", lera, qaWatchableNames{"a1": "Hero", "bd": "Bard"})
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{"a1": "Hero", "pm": "Wanderer", "bd": "Bard"})
	qaWatchableRequireChange(t, "a player who controls none", ivo,
		qaWatchableNames{"a1": "Hero", "pm": "Wanderer", "bd": "Bard"})

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "pm", ParticipantId: f.playerID,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	qaWatchableRequireChange(t, "the grantee of an unseen party member", lera,
		qaWatchableNames{"a1": "Hero", "bd": "Bard", "pm": "Wanderer"})
	qaWatchableRequireNoSet(t, "the spectator", qaWatchableDrain(spec))
	qaWatchableRequireNoSet(t, "another player", qaWatchableDrain(ivo))

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: participantID(t, f, f.otherPlayerToken),
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	qaWatchableRequireMove(t, "a player granted a first party member", ivo, qaWatchableNames{"a1": "Hero"}, "a1")
	qaWatchableRequireNoSet(t, "the co-controller", qaWatchableDrain(lera))
	qaWatchableRequireNoSet(t, "the spectator", qaWatchableDrain(spec))
}

// VTT-400
func TestQAWatchableAPartyMemberWithNoControllerJoinsOnlyTheSpectatorsSet(t *testing.T) {
	f := newGWFixture(t)
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero"})
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero"})

	appendEvents(t, f.campaign, qaWatchableUncontrolled("pm", "Wanderer"))
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{"a1": "Hero", "pm": "Wanderer"})
	qaWatchableRequireNoSet(t, "a player", qaWatchableDrain(lera))
}

// VTT-400 VTT-456
func TestQAWatchableARevocationChangesOnlyTheRevokedPlayersSet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, qaWatchableMember("a2", "Ash", f.playerID)...)
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero", "a2": "Ash"})

	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a2", ParticipantId: f.playerID})
	qaWatchableRequireChange(t, "the revoked player", lera, qaWatchableNames{"a1": "Hero"})
	qaWatchableRequireNoSet(t, "the spectator", qaWatchableDrain(spec))

	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID})
	qaWatchableRequireChange(t, "the player revoked their last", lera, qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	qaWatchableRequireNoSet(t, "the spectator", qaWatchableDrain(spec))
}

// VTT-400; the set comes before the ViewpointChanged it causes (SPEC-015)
func TestQAWatchableANonPartyGrantTakesTheActorOutOfEverySet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, qaWatchableMember("a2", "Ash", f.playerID)...)
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	ivo := f.watch(f.otherPlayerToken, 0, "")
	qaWatchableOpenLog(t, f, ivo, "", qaWatchableNames{"a1": "Hero", "a2": "Ash"})

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: f.playerID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	qaWatchableRequireChange(t, "the controller", lera, qaWatchableNames{"a1": "Hero"})
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{"a1": "Hero"})
	qaWatchableRequireChange(t, "another player", ivo, qaWatchableNames{"a1": "Hero"})

	appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: f.playerID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	qaWatchableRequireMove(t, "the controller of the watched one", lera, qaWatchableNames{}, "")
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{})
	qaWatchableRequireChange(t, "another player", ivo, qaWatchableNames{})
}

// VTT-400; the set comes before the ViewpointChanged it causes (SPEC-015)
func TestQAWatchableARemovalTakesTheActorOutOfEverySet(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, qaWatchableMember("a2", "Ash", f.playerID)...)
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	spec := f.watch(f.spectatorToken, 0, "a1")
	qaWatchableOpenLog(t, f, spec, "a1", qaWatchableNames{"a1": "Hero", "a2": "Ash"})

	appendEvents(t, f.campaign, &vttv1.TokenRemoved{TokenId: "t1"}, &vttv1.ActorRemoved{ActorId: "a1"})
	qaWatchableRequireMove(t, "the player watching the removed one", lera, qaWatchableNames{"a2": "Ash"}, "a2")
	qaWatchableRequireMove(t, "the spectator on its shoulder", spec, qaWatchableNames{"a2": "Ash"}, "")
}

// VTT-400, VTT-399
func TestQAWatchableAnActorAddedAgainIsNamedByItsNewName(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, qaWatchableMember("a2", "Ash", f.playerID)...)
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero", "a2": "Ash"})

	appendEvents(t, f.campaign, &vttv1.ActorRemoved{ActorId: "a2"})
	qaWatchableRequireChange(t, "the player", lera, qaWatchableNames{"a1": "Hero"})
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{"a1": "Hero"})

	appendEvents(t, f.campaign, qaWatchableMember("a2", "Ember", f.playerID)...)
	qaWatchableRequireChange(t, "the player", lera, qaWatchableNames{"a1": "Hero", "a2": "Ember"})
	qaWatchableRequireChange(t, "the spectator", spec, qaWatchableNames{"a1": "Hero", "a2": "Ember"})
	qaWatchableOpenLog(t, f, f.watch(f.playerToken, 0, ""), "a1", qaWatchableNames{"a1": "Hero", "a2": "Ember"})
}

// VTT-400; the set comes before the ViewpointChanged it causes (SPEC-015)
func TestQAWatchableAPromotionChangesTheSet(t *testing.T) {
	t.Run("a spectator made a player", func(t *testing.T) {
		f := newGWFixture(t)
		dm := f.watch(f.dmToken, 0, "")
		id := participantID(t, f, f.spectatorToken)
		appendEvents(t, f.campaign, qaWatchableMember("c1", "Cole", id)...)
		appendEvents(t, f.campaign, qaWatchableMember("b1", "Bryn", participantID(t, f, f.otherPlayerToken))...)
		spec := f.watch(f.spectatorToken, 0, "")
		qaWatchableOpenLog(t, f, spec, "", qaWatchableNames{"a1": "Hero", "b1": "Bryn", "c1": "Cole"})

		promote(t, dm, id, identity.RolePlayer)
		qaWatchableRequireMove(t, "the promoted spectator", spec, qaWatchableNames{"c1": "Cole"}, "c1")
	})
	t.Run("a perched spectator made a player who holds nothing keeps its set and its perch", func(t *testing.T) {
		f := newGWFixture(t)
		dm := f.watch(f.dmToken, 0, "")
		spec := f.watch(f.spectatorToken, 0, "a1")
		qaWatchableOpenLog(t, f, spec, "a1", qaWatchableNames{"a1": "Hero"})

		promote(t, dm, participantID(t, f, f.spectatorToken), identity.RolePlayer)
		qaWatchableRequireNoSet(t, "the promoted spectator", qaWatchableDrain(spec))
		appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "after the promotion"})
		frames := qaWatchableUntil(spec, "the narration's entry", func(fr *vttv1.ServerFrame) bool { return fr.GetEvent() != nil })
		if i := qaWatchableIndex(frames, qaWatchableIsViewpoint); i >= 0 {
			t.Fatalf("the promotion moved the seat: %v", frames[i])
		}
	})
	t.Run("a player made a spectator", func(t *testing.T) {
		f := newGWFixture(t)
		dm := f.watch(f.dmToken, 0, "")
		appendEvents(t, f.campaign, qaWatchableMember("b1", "Bryn", participantID(t, f, f.otherPlayerToken))...)
		lera := f.watch(f.playerToken, 0, "")
		qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero"})

		promote(t, dm, f.playerID, identity.RoleSpectator)
		qaWatchableRequireChange(t, "the demoted player", lera, qaWatchableNames{"a1": "Hero", "b1": "Bryn"})
	})
}

// VTT-399, VTT-400
func TestQAWatchableEveryConnectionOfAParticipantIsSentTheSet(t *testing.T) {
	f := newGWFixture(t)
	devices := []*watchConn{f.watch(f.playerToken, 0, ""), f.watch(f.playerToken, 0, "")}
	for _, w := range devices {
		qaWatchableOpenLog(t, f, w, "a1", qaWatchableNames{"a1": "Hero"})
	}
	appendEvents(t, f.campaign, qaWatchableMember("bd", "Bard", f.playerID)...)
	for _, w := range devices {
		qaWatchableRequireChange(t, "a device", w, qaWatchableNames{"a1": "Hero", "bd": "Bard"})
	}
}

// No set without a change: a set is queued when it differs (SPEC-015)
func TestQAWatchableAChangeThatLeavesTheSetAloneSendsNone(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	appendEvents(t, f.campaign, qaWatchableMember("a0", "Zed", f.playerID)...)
	lera := f.watch(f.playerToken, 0, "a1")
	both := qaWatchableNames{"a0": "Zed", "a1": "Hero"}
	qaWatchableOpenLog(t, f, lera, "a1", both)
	spec := f.watch(f.spectatorToken, 0, "")
	qaWatchableOpenLog(t, f, spec, "", both)
	ivo := f.watch(f.otherPlayerToken, 0, "")
	qaWatchableOpenLog(t, f, ivo, "", both)

	quietAfterEntry := func(what string, payloads ...proto.Message) {
		t.Helper()
		n := len(logOf(t, f, "a1"))
		appendEvents(t, f.campaign, payloads...)
		frames := qaWatchableUntil(lera, what+"'s entry", func(fr *vttv1.ServerFrame) bool { return fr.GetEvent() != nil })
		frames = append(frames, qaWatchableDrain(lera)...)
		if len(logOf(t, f, "a1")) == n {
			t.Fatalf("%s wrote nothing to a1's log", what)
		}
		qaWatchableRequireNoSet(t, "the player after "+what, frames)
		qaWatchableRequireNoSet(t, "the spectator after "+what, qaWatchableDrain(spec))
		qaWatchableRequireNoSet(t, "another player after "+what, qaWatchableDrain(ivo))
	}
	quietAfterEntry("a narration", &vttv1.NarrationAdded{Text: "the cave drips"})
	quietAfterEntry("a token move", &vttv1.TokenMoved{TokenId: "t1", SceneId: "scn1",
		From: &vttv1.GridPosition{X: 3, Y: 7}, To: &vttv1.GridPosition{X: 4, Y: 7}})
	quietAfterEntry("a non-party actor in the player's control",
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Gob", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "gob", ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.NarrationAdded{Text: "a goblin joins"})

	if r := lera.command(setViewpoint("a0")); !r.GetOk() {
		t.Fatalf("switching to a0 was refused: %s", r.GetError())
	}
	frames := qaWatchableUntil(lera, "the switch", qaWatchableIsViewpoint)
	qaWatchableRequireNoSet(t, "the player after a switch", append(frames, qaWatchableDrain(lera)...))
	if r := spec.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("perching on a1 was refused: %s", r.GetError())
	}
	frames = qaWatchableUntil(spec, "the perch", qaWatchableIsViewpoint)
	qaWatchableRequireNoSet(t, "the spectator after a perch", append(frames, qaWatchableDrain(spec)...))

	promote(t, dm, f.playerID, identity.RoleSpectator)
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "after the promotion"})
	frames = qaWatchableUntil(lera, "the narration's entry", func(fr *vttv1.ServerFrame) bool { return fr.GetEvent() != nil })
	qaWatchableRequireNoSet(t, "a player made a spectator of the same set", append(frames, qaWatchableDrain(lera)...))
}

// VTT-401
func TestQAWatchableTheDMAndTheAgentAreSentNoSet(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	agent := f.watch(f.agentToken, 0, "")
	lera := f.watch(f.playerToken, 0, "")
	qaWatchableOpenLog(t, f, lera, "a1", qaWatchableNames{"a1": "Hero"})

	grant := appendEvents(t, f.campaign, qaWatchableMember("a2", "Ash", f.playerID)...)
	qaWatchableRequireChange(t, "the player, as a control", lera, qaWatchableNames{"a1": "Hero", "a2": "Ash"})
	appendEvents(t, f.campaign, qaWatchableUncontrolled("pm", "Wanderer"))
	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a2", ParticipantId: f.playerID})
	appendEvents(t, f.campaign, &vttv1.TokenRemoved{TokenId: "t1"}, &vttv1.ActorRemoved{ActorId: "a1"})
	promote(t, dm, participantID(t, f, f.spectatorToken), identity.RolePlayer)

	for name, w := range map[string]*watchConn{"the DM": dm, "the agent": agent} {
		frames := qaWatchableDrain(w)
		live := false
		for _, fr := range frames {
			if fr.GetEvent().GetEventId() == grant[1].GetEventId() {
				live = true
			}
		}
		if !live {
			t.Fatalf("%s was not sent the grant, so its silence proves nothing: %v", name, frames)
		}
		if i := qaWatchableIndex(frames, qaWatchableIsSet); i >= 0 {
			t.Fatalf("%s was sent a set: %v", name, frames[i])
		}
	}
}
