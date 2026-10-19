package gateway_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func grantKind(actorID, participantID string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: actorID, ParticipantId: participantID, Kind: kind}}}
}

// VTT-316 VTT-317 VTT-318 VTT-321 VTT-326 VTT-340 VTT-342 VTT-343 VTT-350 VTT-391
func TestACreatureGrantedToAPlayerAsAPartyMemberIsRunAsOne(t *testing.T) {
	f := newGWFixture(t)
	ivo := participantID(t, f, f.otherPlayerToken)
	appendTwoRooms(t, f)
	appendEvents(t, f.campaign, partyMemberOf("ana", ivo)...)
	appendEvents(t, f.campaign, tokenAt("tok-ana", "ana", 1, 1))
	appendEvents(t, f.campaign, &vttv1.NoteUpserted{Key: "lore", Title: "Lore", Text: "a public fact",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wolf", Name: "Wolf", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		tokenAt("tok-wolf", "wolf", 5, 1))

	dm := f.watch(f.dmToken, 0, "")
	w := f.watch(f.otherPlayerToken, 0, "")
	if _, vp := w.opening(); vp != "ana" {
		t.Fatalf("A0: the connection opens on %q, want ana", vp)
	}
	seen := len(logOf(t, f, "ana"))
	w.entries(seen)
	if n := len(logOf(t, f, "wolf")); n != 0 {
		t.Fatalf("A0: the wolf has %d entries before it is granted", n)
	}

	if r := dm.command(grantKind("wolf", ivo, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)); !r.GetOk() {
		t.Fatalf("A1: the party-member grant is refused: %s", r.GetError())
	}
	ana := logOf(t, f, "ana")
	w.entries(len(ana) - seen)
	if r := w.command(setViewpoint("wolf")); !r.GetOk() {
		t.Fatalf("A1: set_viewpoint on the wolf is refused: %s", r.GetError())
	}
	if vp, before := w.untilViewpoint(); vp != "wolf" || len(before) != 0 {
		t.Fatalf("A1: switched to %q after %d entries, want the wolf after none", vp, len(before))
	}
	opened := logOf(t, f, "wolf")
	requireLog(t, "A1: the wolf's log", w.entries(len(opened)), opened)
	if len(opened) == 0 || opened[0].GetSequence() != 1 || opened[0].GetSessionStarted() == nil {
		t.Fatalf("A1: the wolf's log does not open with the session at 1: %v", opened)
	}

	if r := w.command(moveTo("tok-wolf", 4, 1)); !r.GetOk() {
		t.Fatalf("A2: a move onto (4,1), which the wolf sees and ana does not, is refused: %s", r.GetError())
	}
	if r := w.command(moveTo("tok-wolf", 2, 1)); r.GetOk() || r.GetError() != unseenSquare {
		t.Fatalf("A2: a move onto (2,1), which ana sees and the wolf does not: ok=%v %q", r.GetOk(), r.GetError())
	}
	moved := logOf(t, f, "wolf")
	requireLog(t, "A2: the move's entries", w.entries(len(moved)-len(opened)), moved[len(opened):])

	if r := dm.command(grantKind("wolf", ivo, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)); !r.GetOk() {
		t.Fatalf("A3: the non-party grant is refused: %s", r.GetError())
	}
	if vp, before := w.untilViewpoint(); vp != "ana" || len(before) != 0 {
		t.Fatalf("A3: after the run ends the connection watches %q after %d entries, want ana after none", vp, len(before))
	}
	ana = logOf(t, f, "ana")
	requireLog(t, "A3: ana's log from its first entry", w.entries(len(ana)), ana)
	if r := w.command(setViewpoint("wolf")); r.GetOk() {
		t.Fatalf("A3: set_viewpoint on the wolf, no longer a party member, is accepted")
	}
	if paused := logOf(t, f, "wolf"); len(paused) != len(moved) {
		t.Fatalf("A3: the non-party grant wrote %d entries to the wolf's log", len(paused)-len(moved))
	}

	if r := dm.command(grantKind("wolf", ivo, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)); !r.GetOk() {
		t.Fatalf("A4: the second party-member grant is refused: %s", r.GetError())
	}
	w.entries(len(logOf(t, f, "ana")) - len(ana))
	if r := w.command(setViewpoint("wolf")); !r.GetOk() {
		t.Fatalf("A4: set_viewpoint on the wolf again is refused: %s", r.GetError())
	}
	if vp, before := w.untilViewpoint(); vp != "wolf" || len(before) != 0 {
		t.Fatalf("A4: switched to %q after %d entries, want the wolf after none", vp, len(before))
	}
	resumed := logOf(t, f, "wolf")
	got := w.entries(len(resumed))
	requireLog(t, "A4: the wolf's log again", got, resumed)
	if len(resumed) <= len(moved) {
		t.Fatalf("A4: the resume wrote nothing: %d entries, paused at %d", len(resumed), len(moved))
	}
	for i := range moved {
		if !proto.Equal(got[i], moved[i]) {
			t.Fatalf("A4: entry %d differs from what the connection was first sent", i+1)
		}
	}
	sessions, lore := 0, 0
	for i, e := range got {
		if e.GetSequence() != int64(i+1) {
			t.Fatalf("A4: entry %d carries sequence %d", i+1, e.GetSequence())
		}
		if e.GetSessionStarted() != nil {
			sessions++
		}
		if e.GetNoteUpserted().GetKey() == "lore" {
			lore++
		}
	}
	if sessions != 1 || lore != 1 {
		t.Fatalf("A4: %d SessionStarted and %d NoteUpserted of lore, want one opening's worth of each", sessions, lore)
	}
	w.quiet(300 * time.Millisecond)
}
