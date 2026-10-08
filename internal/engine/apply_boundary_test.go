package engine_test

import (
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// At-cap accept tests: every size cap in Apply is written `> max`, so a value
// of EXACTLY max must be accepted. TestApplyRejections covers the exceeding
// side of each cap; this file covers the accepting side, pinning each
// comparison at `>` rather than `>=`.
//
// PROVENANCE (do not delete these without re-running the mutation audit):
// gremlins found five surviving CONDITIONALS_BOUNDARY mutants in apply.go
// (:174:40, :189:24, :202:38, :205:20, :208:40) — each flipping a cap from
// `>` to `>=` with no test noticing. The gap was ledgered as a carry-forward
// at the world-layer merge gate ("multibyte/at-cap boundary tests") and went
// unpaid; separately, TestNarrationAddedAsAtCapIsAccepted's doc comment
// asserted the note key/title/text caps "already follow implicitly via their
// own accept tests" — the surviving mutants show that claim was untrue.
//
// The two remaining survivors in ResourceChanged (:141:15 `computed < 0` and
// :144:30 `computed > int64(res.Max)`) are EQUIVALENT, not gaps: flipping
// either to its inclusive form assigns the value it already holds. They are
// unkillable by construction and must not be "fixed" with a test.

func TestNarrationTextAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text: strings.Repeat("a", 8192),
	})))

	if !reflect.DeepEqual(before, st.Snapshot()) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

// TestNarrationSingleEventAnchorIsAccepted pins from == to as valid. This is
// not an exotic edge: it is the most ordinary anchoring case there is —
// narration describing exactly one preceding event. The `>` in
// `AnchorFromSeq > AnchorToSeq` is what permits it.
func TestNarrationSingleEventAnchorIsAccepted(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text:          "The cutter's blade turns on the shield boss.",
		AnchorFromSeq: 5, AnchorToSeq: 5,
	})))

	if !reflect.DeepEqual(before, st.Snapshot()) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

func TestNoteKeyAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	key := strings.Repeat("k", 128)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: key, Title: "at cap", Text: "body",
	})))

	if _, ok := st.Notes[key]; !ok {
		t.Fatalf("note with a 128-byte key must be stored")
	}
}

func TestNoteTitleAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	title := strings.Repeat("t", 256)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: "k", Title: title, Text: "body",
	})))

	if got := st.Notes["k"].Title; got != title {
		t.Fatalf("title: got %d bytes, want %d", len(got), len(title))
	}
}

func TestNoteTextAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	text := strings.Repeat("x", 8192)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: "k", Title: "at cap", Text: text,
	})))

	if got := st.Notes["k"].Text; got != text {
		t.Fatalf("text: got %d bytes, want %d", len(got), len(text))
	}
}

// VTT-264
func TestMoveReasonAtCapIsAccepted(t *testing.T) {
	st := seedMovableToken(t)

	must(t, engine.Apply(st, env(5, &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn", To: &vttv1.GridPosition{X: 4, Y: 4},
		Reason: strings.Repeat("r", 256),
	})))

	if tok := st.Tokens["t1"]; tok.X != 4 || tok.Y != 4 {
		t.Fatalf("a move with a 256-byte reason must land, token at %d,%d", tok.X, tok.Y)
	}
}

// VTT-274
func TestNamesAtCapAreAccepted(t *testing.T) {
	name := strings.Repeat("n", 256)
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: name})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{SceneId: "scn", Name: name, GridWidth: 2, GridHeight: 2})))
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: name}})))
	must(t, engine.Apply(st, env(4, &vttv1.AdventureLoaded{AdventureId: "adv", Name: name})))
	if got := st.Scenes["scn"].Name; got != name {
		t.Fatalf("the scene's 256-byte name was stored as %d bytes", len(got))
	}
	if got := st.Actors["a1"].GetName(); got != name {
		t.Fatalf("the actor's 256-byte name was stored as %d bytes", len(got))
	}
}

// VTT-277
func TestIDsAtCapAreAccepted(t *testing.T) {
	id := func(c string) string { return strings.Repeat(c, 128) }
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{SceneId: id("s"), Name: "S", GridWidth: 2, GridHeight: 2})))
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id("a")}})))
	must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{TokenId: id("t"), SceneId: id("s"), ActorId: id("a"),
		Position: &vttv1.GridPosition{X: 1, Y: 1}})))
	must(t, engine.Apply(st, env(5, &vttv1.AdventureLoaded{AdventureId: id("v"), Name: "A"})))
	if tok, ok := st.Tokens[id("t")]; !ok || tok.SceneID != id("s") || tok.ActorID != id("a") {
		t.Fatalf("the 128-byte token was not stored whole: %+v ok=%v", tok, ok)
	}
}

// VTT-285
func TestConditionAndAbilityIDsAtCapAreAccepted(t *testing.T) {
	condition, ability := strings.Repeat("é", 64), strings.Repeat("b", 128)
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1"}})))
	must(t, engine.Apply(st, env(2, &vttv1.ConditionApplied{ActorId: "a1", ConditionId: condition})))
	must(t, engine.Apply(st, env(3, &vttv1.AbilityUsed{ActorId: "a1", AbilityId: ability})))
	if got := st.Conditions["a1"]; len(got) != 1 || got[0].ID != condition {
		t.Fatalf("the 128-byte condition was not stored whole: %+v", got)
	}
}

// VTT-289 VTT-290 VTT-291 VTT-293
func TestCommandAndFileIDsAtCapAreAccepted(t *testing.T) {
	id := func(c string) string { return strings.Repeat(c, 128) }
	object := strings.Repeat("é", 64)
	st := engine.NewState()
	started := env(1, &vttv1.SessionStarted{Name: "n"})
	started.SessionId = id("s")
	must(t, engine.Apply(st, started))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 2, GridHeight: 2,
		Objects: []*vttv1.SceneObject{{ObjectId: object, Kind: "k", At: &vttv1.GridPosition{}, Width: 1, Height: 1}}})))
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", ModuleId: id("m"),
		Resources:  map[string]*vttv1.Resource{id("r"): {Current: 1, Max: 1}},
		Attributes: map[string]int32{id("t"): 1}}})))
	must(t, engine.Apply(st, env(4, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: id("p"),
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})))
	if got := st.Actors["a1"].GetControllerIds(); len(got) != 1 || got[0] != id("p") {
		t.Fatalf("the 128-byte participant was not stored whole: %v", got)
	}
	must(t, engine.Apply(st, env(5, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: id("p")})))
	must(t, engine.Apply(st, env(6, &vttv1.TokenPlaced{TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 1, Y: 1}})))
	must(t, engine.Apply(st, env(7, &vttv1.TokenMoved{TokenId: "t1", SceneId: id("v"), To: &vttv1.GridPosition{}})))
	a := st.Actors["a1"]
	if st.Sessions[0].ID != id("s") || st.Scenes["scn"].Objects[0].ObjectID != object || a.GetModuleId() != id("m") ||
		a.GetResources()[id("r")] == nil || a.GetAttributes()[id("t")] != 1 || len(a.GetControllerIds()) != 0 {
		t.Fatalf("the 128-byte values were not stored whole")
	}
}
