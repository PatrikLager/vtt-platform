package engine_test

import (
	"encoding/json"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func qattScene(seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "s1", Name: "Crypt", GridWidth: 10, GridHeight: 10,
	}}}
}

func qattActor(seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Goblin"},
	}}}
}

func qattPlace(seq int64, id string, x, y int32, name, notes, gmName, gmNotes string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: id, SceneId: "s1", ActorId: "a1", Position: &vttv1.GridPosition{X: x, Y: y},
		Name: name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
	}}}
}

func qattDescribe(seq int64, id, name, notes, gmName, gmNotes string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_TokenDescribed{TokenDescribed: &vttv1.TokenDescribed{
		TokenId: id, Name: name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
	}}}
}

func qattApply(t *testing.T, st *engine.State, envs ...*vttv1.Envelope) {
	t.Helper()
	for _, e := range envs {
		if err := engine.Apply(st, e); err != nil {
			t.Fatalf("Apply(seq %d): %v", e.GetSequence(), err)
		}
	}
}

func qattToken(t *testing.T, st *engine.State, id string) engine.Token {
	t.Helper()
	tok, ok := st.Tokens[id]
	if !ok {
		t.Fatalf("token %q missing from state", id)
	}
	return tok
}

func qattDump(t *testing.T, st *engine.State) string {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestQATTAPlacementWithAllFourTextsHoldsThem(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."))
	got := qattToken(t, st, "t1")
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2,
		Name: "Grub", Notes: "Limps.", GMName: "Grub the Spy", GMNotes: "Reports to the lich."}
	if got != want {
		t.Fatalf("token = %+v, want %+v", got, want)
	}
}

func TestQATTAPlacementWithNoTextsHoldsEmptyTexts(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2), qattPlace(3, "t2", 3, 4, "", "", "", ""))
	got := qattToken(t, st, "t2")
	want := engine.Token{ID: "t2", SceneID: "s1", ActorID: "a1", X: 3, Y: 4}
	if got != want {
		t.Fatalf("token = %+v, want %+v", got, want)
	}
}

// VTT-479
func TestQATTDescribeReplacesAllFourAnEmptiedOneIncluded(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		qattDescribe(4, "t1", "", "Now runs.", "", "Turned."))
	got := qattToken(t, st, "t1")
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2,
		Name: "", Notes: "Now runs.", GMName: "", GMNotes: "Turned."}
	if got != want {
		t.Fatalf("after describe token = %+v, want %+v", got, want)
	}

	qattApply(t, st, qattDescribe(5, "t1", "Grub", "", "Grub the Spy", ""))
	got = qattToken(t, st, "t1")
	want = engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2,
		Name: "Grub", Notes: "", GMName: "Grub the Spy", GMNotes: ""}
	if got != want {
		t.Fatalf("after second describe token = %+v, want %+v", got, want)
	}
}

// VTT-479
func TestQATTDescribeWithFourEmptyTextsClearsThemAll(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		qattDescribe(4, "t1", "", "", "", ""))
	got := qattToken(t, st, "t1")
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2}
	if got != want {
		t.Fatalf("token = %+v, want %+v", got, want)
	}
}

// VTT-479
func TestQATTDescribeOfAnUntextedTokenSetsAllFour(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t2", 3, 4, "", "", "", ""),
		qattDescribe(4, "t2", "N", "No", "G", "Gn"))
	got := qattToken(t, st, "t2")
	want := engine.Token{ID: "t2", SceneID: "s1", ActorID: "a1", X: 3, Y: 4,
		Name: "N", Notes: "No", GMName: "G", GMNotes: "Gn"}
	if got != want {
		t.Fatalf("token = %+v, want %+v", got, want)
	}
}

// VTT-479
func TestQATTDescribeLeavesOtherTokensAlone(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		qattPlace(4, "t2", 3, 4, "Snag", "Tall.", "Snag the Elder", "Owes a debt."),
		qattDescribe(5, "t1", "", "", "", ""))
	got := qattToken(t, st, "t2")
	want := engine.Token{ID: "t2", SceneID: "s1", ActorID: "a1", X: 3, Y: 4,
		Name: "Snag", Notes: "Tall.", GMName: "Snag the Elder", GMNotes: "Owes a debt."}
	if got != want {
		t.Fatalf("untouched token = %+v, want %+v", got, want)
	}
}

// VTT-480
func TestQATTDescribeNamingNoTokenIsRefusedAndLeavesTheStateUnchanged(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."))
	before := qattDump(t, st)
	err := engine.Apply(st, qattDescribe(4, "ghost", "X", "X", "X", "X"))
	if err == nil {
		t.Fatal("TokenDescribed naming no token was accepted")
	}
	if after := qattDump(t, st); after != before {
		t.Fatalf("state changed on refusal:\nbefore %s\nafter  %s", before, after)
	}
	if _, ok := st.Tokens["ghost"]; ok {
		t.Fatal("refused describe introduced a token")
	}
}

// VTT-480
func TestQATTDescribeWithAnEmptyTokenIDIsRefused(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "", "", ""))
	before := qattDump(t, st)
	if err := engine.Apply(st, qattDescribe(4, "", "X", "", "", "")); err == nil {
		t.Fatal("TokenDescribed with an empty token id was accepted")
	}
	if after := qattDump(t, st); after != before {
		t.Fatalf("state changed on refusal:\nbefore %s\nafter  %s", before, after)
	}
}

// VTT-480
func TestQATTDescribeOfARemovedTokenIsRefused(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "", "", ""),
		&vttv1.Envelope{Sequence: 4, Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "t1"}}})
	before := qattDump(t, st)
	if err := engine.Apply(st, qattDescribe(5, "t1", "Back", "", "", "")); err == nil {
		t.Fatal("TokenDescribed of a removed token was accepted")
	}
	if after := qattDump(t, st); after != before {
		t.Fatalf("state changed on refusal:\nbefore %s\nafter  %s", before, after)
	}
}

func TestQATTASnapshotCarriesTheTexts(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."))
	got := qattToken(t, st.Snapshot(), "t1")
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2,
		Name: "Grub", Notes: "Limps.", GMName: "Grub the Spy", GMNotes: "Reports to the lich."}
	if got != want {
		t.Fatalf("snapshot token = %+v, want %+v", got, want)
	}
}

func TestQATTTextsSurviveAMove(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		&vttv1.Envelope{Sequence: 4, Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: "t1", SceneId: "s1", From: &vttv1.GridPosition{X: 1, Y: 2}, To: &vttv1.GridPosition{X: 2, Y: 2},
		}}})
	got := qattToken(t, st, "t1")
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 2, Y: 2,
		Name: "Grub", Notes: "Limps.", GMName: "Grub the Spy", GMNotes: "Reports to the lich."}
	if got != want {
		t.Fatalf("token after move = %+v, want %+v", got, want)
	}
}

func TestQATTTextsAreNotBoundedInTheFold(t *testing.T) {
	long := strings.Repeat("ab", 64*1024)
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, long, long, long, long),
		qattDescribe(4, "t1", long+"x", long+"y", long+"z", long+"w"))
	got := qattToken(t, st, "t1")
	if got.Name != long+"x" || got.Notes != long+"y" || got.GMName != long+"z" || got.GMNotes != long+"w" {
		t.Fatal("a long text was not held whole")
	}
}

func TestQATTDumpCarriesEachTextOnlyWhenNonEmptyAfterY(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		qattPlace(4, "t2", 3, 4, "", "", "", ""),
		qattPlace(5, "t3", 5, 6, "", "Odd.", "", "Hidden."),
		qattPlace(6, "t4", 7, 8, "Lone", "", "", ""))
	b, err := json.Marshal(st.Tokens)
	if err != nil {
		t.Fatal(err)
	}
	want := `{` +
		`"t1":{"ID":"t1","SceneID":"s1","ActorID":"a1","X":1,"Y":2,"Name":"Grub","Notes":"Limps.","GMName":"Grub the Spy","GMNotes":"Reports to the lich."},` +
		`"t2":{"ID":"t2","SceneID":"s1","ActorID":"a1","X":3,"Y":4},` +
		`"t3":{"ID":"t3","SceneID":"s1","ActorID":"a1","X":5,"Y":6,"Notes":"Odd.","GMNotes":"Hidden."},` +
		`"t4":{"ID":"t4","SceneID":"s1","ActorID":"a1","X":7,"Y":8,"Name":"Lone"}` +
		`}`
	if string(b) != want {
		t.Fatalf("tokens dump\n got %s\nwant %s", b, want)
	}
}

// qattParityLog is the log client/test/qa-token-texts.test.ts's parityLog folds.
func qattParityLog() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qattScene(1), qattActor(2),
		qattPlace(3, "t1", 1, 2, "Grub", "Limps.", "Grub the Spy", "Reports to the lich."),
		qattPlace(4, "t2", 3, 4, "", "", "", ""),
		qattPlace(5, "t3", 5, 6, "", "<trap> & \"pit\"", "Åsa\u2028x", ""),
		qattDescribe(6, "t1", "", "Now runs.", "", "Turned."),
	}
}

// qattParityDump is encoding/json of engine.State after qattParityLog.
const qattParityDump = `{"Scenes":{"s1":{"ID":"s1","Name":"Crypt","GridWidth":10,"GridHeight":10,"Tiles":{},"Objects":[],"OpenDoors":{}}},"Actors":{"a1":{"actor_id":"a1","name":"Goblin"}},"Tokens":{"t1":{"ID":"t1","SceneID":"s1","ActorID":"a1","X":1,"Y":2,"Notes":"Now runs.","GMNotes":"Turned."},"t2":{"ID":"t2","SceneID":"s1","ActorID":"a1","X":3,"Y":4},"t3":{"ID":"t3","SceneID":"s1","ActorID":"a1","X":5,"Y":6,"Notes":"\u003ctrap\u003e \u0026 \"pit\"","GMName":"Åsa\u2028x"}},"Sessions":null,"Conditions":{},"Notes":{}}`

func TestQATTDumpMatchesTheSharedBytes(t *testing.T) {
	st := engine.NewState()
	qattApply(t, st, qattParityLog()...)
	got := qattDump(t, st)
	if got != qattParityDump {
		t.Fatalf("dump\n got %s\nwant %s", got, qattParityDump)
	}
}
