package gateway_test

// remove_actor_test.go covers the ONE command in the retraction-leaves arc
// that emits a whole ordered batch (Task 9): handleRemoveActor's
// tokens-then-actor cascade, its atomicity, and the property the cascade
// exists for — that the log it leaves behind still folds.
//
// WHY THE CASCADE IS CORRECTNESS AND NOT CONVENIENCE. engine.Apply's
// TokenPlaced arm and client/src/fold.ts's tokenPlaced arm both refuse a token
// whose actor they do not know, in almost the same words. An ActorRemoved that
// left this actor's tokens standing would leave a world whose own
// introductions no longer fold, which through client/src/session.ts's
// re-fold-the-whole-log-on-every-event is the permanent client freeze this
// whole sub-project exists to make impossible.
//
// load_map is the precedent for the shape (map_test.go, handleLoadMap): an
// ordered batch handed to campaign.AppendBatch, accepted or rejected whole.

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// removeActorCmdFor builds a RemoveActor ClientCommand for id.
func removeActorCmdFor(requestID, id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID,
		Command: &vttv1.ClientCommand_RemoveActor{RemoveActor: &vttv1.RemoveActor{ActorId: id}}}
}

// removalPayloadKind names env's oneof payload variant, restricted to the two
// variants remove_actor's batch can ever contain — mapPayloadKind's sibling
// (map_test.go), same idiom.
func removalPayloadKind(env *vttv1.Envelope) string {
	switch env.Payload.(type) {
	case *vttv1.Envelope_TokenRemoved:
		return "tokenRemoved"
	case *vttv1.Envelope_ActorRemoved:
		return "actorRemoved"
	default:
		return "other"
	}
}

// TestRemoveActorEmitsEveryTokenThenTheActor pins the batch's CONTENT and its
// ORDER: one TokenRemoved per token of that actor, in token-id order, and then
// the ActorRemoved, last.
//
// The second token is placed with an id that sorts BEFORE the seeded one
// ("t0" < "t1") while arriving after it, so the assertion below distinguishes
// "sorted by id" from "whatever order the map iterated" — which for a Go map
// is deliberately randomised, and would otherwise make this test pass or fail
// by coin toss.
func TestRemoveActorEmitsEveryTokenThenTheActor(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 0)

	sendCommand(t, dmConn, &vttv1.ClientCommand{RequestId: "seed-t0",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "t0", SceneId: "scn1", ActorId: "a1",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}}})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed place_token t0: %s", r.Error)
	}

	// Read the batch back off a SECOND connection, dialled HERE — after the
	// seeding, at the current head — so that everything it ever receives is
	// this command's batch. That is the whole reason for it: newGWFixture
	// seeds scn1/a1/t1, and dmConn resumed from 0, so dmConn's event queue
	// already holds that history plus the place_token broadcast just made,
	// and reading positionally off it would assert against the seed.
	//
	// NOT because readResult would swallow anything. It would not: the
	// helpers run a per-connection demultiplexer with separate result and
	// event channels, and readResult's own doc comment says "Events that
	// arrive first are queued, not discarded, so a later readEvent still sees
	// them". The claim that it discards them survives in stale comments
	// elsewhere in this package and is corrected there too (fix round 1, I3).
	//
	// The AGENT seat, not the player: the agent receives the log unfiltered
	// (spec §3.1, exit criterion 8), and a projected player's stream is a
	// different thing with its own tests (project_test.go).
	agentConn := f.dial(f.agentToken, f.head(t))

	sendCommand(t, dmConn, removeActorCmdFor("r-remove", "a1"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("want ok=true removing actor a1, got %+v", res)
	}

	want := []struct {
		kind string
		id   string
	}{
		{"tokenRemoved", "t0"},
		{"tokenRemoved", "t1"},
		{"actorRemoved", "a1"},
	}
	for i, w := range want {
		env := readEvent(t, agentConn)
		if got := removalPayloadKind(env); got != w.kind {
			t.Fatalf("batch envelope %d kind = %q, want %q", i, got, w.kind)
		}
		got := env.GetActorRemoved().GetActorId()
		if w.kind == "tokenRemoved" {
			got = env.GetTokenRemoved().GetTokenId()
		}
		if got != w.id {
			t.Fatalf("batch envelope %d subject = %q, want %q", i, got, w.id)
		}
		// Contiguous, in this order, starting at the sequence the result
		// named: the batch is one AppendBatch call, so nothing can be
		// interleaved into it.
		if wantSeq := res.GetSequence() + int64(i); env.GetSequence() != wantSeq {
			t.Fatalf("batch envelope %d sequence = %d, want %d", i, env.GetSequence(), wantSeq)
		}
	}

	if st := f.campaign.State(); st != nil {
		if _, ok := st.Actors["a1"]; ok {
			t.Fatal("want actor a1 gone from folded state")
		}
		for _, id := range []string{"t0", "t1"} {
			if _, ok := st.Tokens[id]; ok {
				t.Fatalf("want token %q gone from folded state", id)
			}
		}
	}
}

// TestARemovedActorLeavesALogThatStillFolds is the property the cascade exists
// for, and the only one of these tests that would catch the cascade being
// dropped altogether: the log is folded AGAIN, from nothing, by a second
// Campaign opened on the same file — campaign.Open rebuilds state by replaying
// every event through engine.Apply, which is exactly what a fresh reader (and,
// mirrored, every client) does.
//
// A cascade-free ActorRemoved would still APPEND fine here; it is the replay
// that would be left holding a token whose actor no longer exists.
func TestARemovedActorLeavesALogThatStillFolds(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 0)

	sendCommand(t, dmConn, removeActorCmdFor("r-remove", "a1"))
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("want ok=true removing actor a1, got %+v", r)
	}

	replayed, err := campaign.Open(f.path)
	if err != nil {
		t.Fatalf("reopen campaign (this is the re-fold): %v", err)
	}
	t.Cleanup(func() { replayed.Close() })

	st := replayed.State()
	if st == nil {
		// A poisoned replay would mean the log itself does not fold — which
		// the cascade is NOT what prevents, and this branch is therefore not
		// the assertion this test is about. Kept as a fail-loud guard so a
		// poisoned campaign says so instead of nil-panicking two lines down.
		// The orphan-token check is what catches a missing cascade.
		t.Fatal("replayed campaign is poisoned: engine.Apply refused an event on replay")
	}
	if _, ok := st.Actors["a1"]; ok {
		t.Fatal("want actor a1 absent from the replayed state")
	}
	if _, ok := st.Tokens["t1"]; ok {
		t.Fatal("want a1's token t1 absent from the replayed state — an orphan token is a world whose introductions no longer fold")
	}
}

// TestRemoveActorAppendsNothingWhenThePartyIsUnknown pins the CLEAN REFUSAL: an
// unknown actor comes back as ok=false in the fold's own wording, and the log
// head does not move.
//
// IT IS NOT THE ATOMICITY PROPERTY, and its comment claimed to be until fix
// round 1 (I4). The batch here is ONE event, so there is no partial state for
// atomicity to be about; what this measures is that the handler refuses rather
// than dropping the connection, and that a refusal costs the log nothing.
// TestAMidBatchRefusalPersistsNothingThatCameBeforeIt is the test that applies
// an event and then throws it away.
func TestRemoveActorAppendsNothingWhenThePartyIsUnknown(t *testing.T) {
	f := newGWFixture(t)
	before := f.head(t)
	dmConn := f.dial(f.dmToken, 0)

	sendCommand(t, dmConn, removeActorCmdFor("r-ghost", "nobody"))
	res := readResult(t, dmConn)
	if res.Ok {
		t.Fatal("want ok=false removing an actor that does not exist")
	}
	if res.GetError() != `engine: removed unknown actor "nobody"` {
		t.Fatalf("error = %q, want the fold's own unknown-actor wording", res.GetError())
	}
	if after := f.head(t); after != before {
		t.Fatalf("log head moved from %d to %d — a refused removal must append nothing", before, after)
	}
}

// TestAnOutOfOrderRemovalBatchIsRejectedWhole is what makes the batch's ORDER
// load-bearing rather than merely tidy. The same two events remove_actor emits,
// in the wrong order, are refused as a batch — by the fold itself
// (engine.Apply's ActorRemoved arm refuses an actor whose tokens still stand),
// which campaign.AppendBatch runs over the whole batch BEFORE persisting any of
// it.
//
// IT IS NOT THE ATOMICITY PROPERTY EITHER, and its comment claimed to be until
// fix round 1 (I4): the refusal lands on batch event 0, so again nothing is
// applied and then discarded. Order, not atomicity, is what it proves.
//
// This is also the race that makes that fold guard worth having:
// handleRemoveActor builds its batch from a SNAPSHOT and AppendBatch takes the
// lock afterwards, so a place_token landing in between would otherwise append
// an ActorRemoved that orphans a token. It cannot; it is rejected here.
func TestAnOutOfOrderRemovalBatchIsRejectedWhole(t *testing.T) {
	f := newGWFixture(t)
	before := f.head(t)

	_, err := f.campaign.AppendBatch([]*vttv1.Envelope{
		{EventId: "wrong-1", Payload: &vttv1.Envelope_ActorRemoved{
			ActorRemoved: &vttv1.ActorRemoved{ActorId: "a1"}}},
		{EventId: "wrong-2", Payload: &vttv1.Envelope_TokenRemoved{
			TokenRemoved: &vttv1.TokenRemoved{TokenId: "t1"}}},
	})
	if err == nil {
		t.Fatal("want the actor-before-its-tokens batch rejected")
	}
	// BY THE GUARD, named — not merely "some error". Without this the test
	// would pass while the ActorRemoved arm did not exist at all (the fold
	// rejects an unknown variant too), which proves nothing about ordering.
	if !strings.Contains(err.Error(), "cannot outlive its actor") {
		t.Fatalf("AppendBatch error = %q, want the fold's own token-outlives-actor refusal", err)
	}
	if after := f.head(t); after != before {
		t.Fatalf("log head moved from %d to %d — a rejected batch must append nothing", before, after)
	}
	st := f.campaign.State()
	if st == nil {
		t.Fatal("campaign poisoned by a rejected batch")
	}
	if _, ok := st.Actors["a1"]; !ok {
		t.Fatal("want actor a1 still present after the rejected batch")
	}
	if _, ok := st.Tokens["t1"]; !ok {
		t.Fatal("want token t1 still present after the rejected batch")
	}
}

// TestAMidBatchRefusalPersistsNothingThatCameBeforeIt is the "a partial
// application is impossible" property, MEASURED — and it is the only test here
// that measures it, which fix round 1 (I4) is the reason for.
//
// The two tests written for it first both reject on batch event 0:
// TestRemoveActorAppendsNothingWhenThePartyIsUnknown sends a ONE-event batch,
// where there is no partial state to be impossible, and
// TestAnOutOfOrderRemovalBatchIsRejectedWhole puts the ActorRemoved first. Both
// are worth having for what they do pin — a clean refusal, and the order being
// load-bearing — but neither ever applies an event and then throws it away.
//
// This one does. The batch removes t1 and then a1 while a1 STILL HOLDS t2, so
// event 0 folds successfully against campaign.AppendBatch's validation snapshot
// and event 1 is refused by the ActorRemoved arm's own guard. Nothing may
// persist: not the log head, and not t1, whose removal succeeded inside the
// batch that was then thrown away.
func TestAMidBatchRefusalPersistsNothingThatCameBeforeIt(t *testing.T) {
	f := newGWFixture(t)
	mustAppend(t, f.campaign, "seed-t2", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "t2", SceneId: "scn1", ActorId: "a1", Position: &vttv1.GridPosition{X: 4, Y: 4}}})
	before := f.head(t)

	_, err := f.campaign.AppendBatch([]*vttv1.Envelope{
		{EventId: "half-1", Payload: &vttv1.Envelope_TokenRemoved{
			TokenRemoved: &vttv1.TokenRemoved{TokenId: "t1"}}},
		{EventId: "half-2", Payload: &vttv1.Envelope_ActorRemoved{
			ActorRemoved: &vttv1.ActorRemoved{ActorId: "a1"}}},
	})
	if err == nil {
		t.Fatal("want the batch refused: a1 still holds t2, which the ActorRemoved arm guards against")
	}
	// NAMED, so this cannot pass on an unrelated failure, and naming t2
	// specifically proves event 0 was APPLIED to the validation snapshot: the
	// guard reports the first remaining token in id order, and t1 sorts before
	// t2 — it would have been named had its own removal not folded first.
	if want := `engine: actor "a1" still has token "t2" on the board`; !strings.Contains(err.Error(), want) {
		t.Fatalf("AppendBatch error = %q, want it to contain %q", err, want)
	}

	if after := f.head(t); after != before {
		t.Fatalf("log head moved from %d to %d — a refused batch must append nothing", before, after)
	}
	st := f.campaign.State()
	if st == nil {
		t.Fatal("campaign poisoned by a refused batch")
	}
	// THE HALF THAT MATTERS: t1's removal succeeded inside the batch and must
	// leave no trace at all.
	for _, id := range []string{"t1", "t2"} {
		if _, ok := st.Tokens[id]; !ok {
			t.Fatalf("want token %q still present: the batch that removed it was refused whole", id)
		}
	}
	if _, ok := st.Actors["a1"]; !ok {
		t.Fatal("want actor a1 still present after the refused batch")
	}
}

// TestRemovingAnActorWithNoTokensYieldsABatchOfOne pins the degenerate end of
// the cascade: no tokens means no TokenRemoved events, not an empty batch
// (campaign.AppendBatch refuses one) and not a placeholder.
func TestRemovingAnActorWithNoTokensYieldsABatchOfOne(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, 0)

	sendCommand(t, dmConn, &vttv1.ClientCommand{RequestId: "seed-a2",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "a2", Name: "Goblin",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}}})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed add_actor a2: %s", r.Error)
	}
	before := f.head(t)

	sendCommand(t, dmConn, removeActorCmdFor("r-a2", "a2"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("want ok=true removing actor a2, got %+v", res)
	}
	if after := f.head(t); after != before+1 {
		t.Fatalf("log head moved from %d to %d — an actor with no tokens is ONE event", before, after)
	}
	if res.GetSequence() != before+1 {
		t.Fatalf("result.Sequence = %d, want %d (the batch's first and only event)", res.GetSequence(), before+1)
	}
}
