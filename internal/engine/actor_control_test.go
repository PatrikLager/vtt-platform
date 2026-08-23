package engine_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Actor control is a SET, and `controller_id` is a MIRROR of it —
// controller_ids[0] whenever the set is non-empty, empty only when it is.
//
// The mirror is not a convenience. ActorAdded events carrying controller_id
// exist in real campaign logs, and readers written before the set existed
// still consult it: internal/gateway/authz.go, client/src/player.ts's "your
// actors" filter, `vtt state dump`, MCP get_state. If the fold ever lets the
// two disagree, one of those silently gains or loses a character for someone.
//
// The rejected alternative was "controller_id empty whenever the set has more
// than one". protojson omits empty strings, so a SHARED actor would have been
// byte-identical on the wire to an UNOWNED one — and empty already means
// DM/agent-only. controller_ids[0] leaves an old reader incomplete but never
// wrong, which fails closed.

func grantEnv(seq int64, actorID, participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "grant", Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: participantID},
	}}
}

func revokeEnv(seq int64, actorID, participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "revoke", Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actorID, ParticipantId: participantID},
	}}
}

func resourceChangedEnv(seq int64, actorID string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "res", Payload: &vttv1.Envelope_ResourceChanged{
		ResourceChanged: &vttv1.ResourceChanged{ActorId: actorID, Resource: "vigor", Delta: -1, NewValue: 4},
	}}
}

func actorAddedControlled(seq int64, actorID, controllerID string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "added", Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: actorID, ControllerId: controllerID,
			// A declared resource so ResourceChanged can reach this actor —
			// the whole-fold test needs a non-control event that MUTATES a
			// stored actor, and every other field here is irrelevant to it.
			Resources: map[string]*vttv1.Resource{"vigor": {Current: 5, Max: 5}},
		}},
	}}
}

// foldAll applies envs to a fresh state, failing on the first error.
func foldAll(t *testing.T, envs ...*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, env := range envs {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("Apply[%d]: %v", i, err)
		}
	}
	return st
}

// assertMirror is the invariant, checked after every scenario below rather
// than in one place: a disagreement is only interesting where it appears.
func assertMirror(t *testing.T, st *engine.State, actorID string) {
	t.Helper()
	a, ok := st.Actors[actorID]
	if !ok {
		t.Fatalf("actor %q missing", actorID)
	}
	ids := a.GetControllerIds()
	got := a.GetControllerId()
	switch {
	case len(ids) == 0 && got != "":
		t.Fatalf("actor %q: controller_id %q with an EMPTY set — an old reader sees a controller nobody has",
			actorID, got)
	case len(ids) > 0 && got != ids[0]:
		t.Fatalf("actor %q: controller_id %q but controller_ids[0] is %q — the two disagree, so one reader "+
			"grants and another denies", actorID, got, ids[0])
	case len(ids) > 0 && got == "":
		t.Fatalf("actor %q: NON-EMPTY set %v with an empty controller_id — empty already means "+
			"DM/agent-only, so this reads as unowned to every old reader", actorID, ids)
	}
}

func TestAddActorSeedsTheControlSetFromControllerId(t *testing.T) {
	st := foldAll(t, actorAddedControlled(1, "thorn", "p-player"))
	assertMirror(t, st, "thorn")
	if ids := st.Actors["thorn"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-player" {
		t.Fatalf("want the set seeded with the declared controller, got %v", ids)
	}
}

func TestAddActorWithNoControllerHasAnEmptySet(t *testing.T) {
	// Empty keeps its original meaning: DM/agent only. This is the case every
	// NPC and monster is in, so it is the common one.
	st := foldAll(t, actorAddedControlled(1, "goblin", ""))
	assertMirror(t, st, "goblin")
	if ids := st.Actors["goblin"].GetControllerIds(); len(ids) != 0 {
		t.Fatalf("want an empty set for a controllerless actor, got %v", ids)
	}
}

func TestTheFoldStoresAnActorsKindExactlyAsTheLogWroteIt(t *testing.T) {
	// KIND IS STORED VERBATIM, and the fold applies NO migration to it — that
	// is the half of spec §5.1 that is easy to get backwards, and getting it
	// backwards is silent.
	//
	// The tempting shortcut is to normalise here: absent + a controller
	// becomes PARTY_MEMBER at fold time, and every reader is spared the rule.
	// It would be wrong twice over. First, it makes engine.State say something
	// the LOG never said, and `vtt state dump` is a rendering of the log — the
	// hand-derived scenarios/goldens corpus would have to gain a kind field
	// nobody ever wrote. Second, it decides for every future reader what
	// absence means, permanently, in an append-only contract. The reading
	// belongs to the readers (gateway.isPartyMember), and the fold's job is to
	// keep the log's own words.
	//
	// Also the plainest statement that Go's Apply and client/src/fold.ts stay
	// strict mirrors on this field: copyActor in fold.ts enumerates fields by
	// hand, so it can drop one silently where proto.Clone cannot. Its twin is
	// client/test/fold-unit.test.ts.
	for _, tc := range []struct {
		name string
		set  []string
		kind vttv1.ActorKind
		want vttv1.ActorKind
	}{
		{"declared party member", []string{"p-player"}, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"declared non-party", nil, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		// A monster somebody holds: the leak §5.1 closes, and the case that
		// proves the fold does not quietly re-derive kind from control.
		{"declared non-party but controlled", []string{"dm-1"}, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		// The two migration shapes. Both must come back UNSPECIFIED: the log
		// said nothing, so the state says nothing.
		{"undeclared and controlled", []string{"p-player"}, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED},
		{"undeclared and uncontrolled", nil, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := foldAll(t, &vttv1.Envelope{Sequence: 1, EventId: "added",
				Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
					Actor: &vttv1.Actor{ActorId: "a", ControllerIds: tc.set, Kind: tc.kind}}}})
			if got := st.Actors["a"].GetKind(); got != tc.want {
				t.Fatalf("want the fold to store %v verbatim, got %v", tc.want, got)
			}
		})
	}
}

func TestGrantingASecondControllerKeepsTheFirstInControllerId(t *testing.T) {
	// THE case the rejected rule got wrong. p-player must still see Thorn as
	// theirs after someone else is added.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		grantEnv(2, "thorn", "p-second"),
	)
	assertMirror(t, st, "thorn")
	a := st.Actors["thorn"]
	if got := a.GetControllerId(); got != "p-player" {
		t.Fatalf("controller_id = %q, want the original controller retained — blanking it here is what "+
			"would drop the character from its own player's list", got)
	}
	if ids := a.GetControllerIds(); len(ids) != 2 || ids[0] != "p-player" || ids[1] != "p-second" {
		t.Fatalf("want both controllers in insertion order, got %v", ids)
	}
}

func TestGrantingToAnActorWithNoControllerFillsControllerId(t *testing.T) {
	st := foldAll(t,
		actorAddedControlled(1, "goblin", ""),
		grantEnv(2, "goblin", "p-player"),
	)
	assertMirror(t, st, "goblin")
	if got := st.Actors["goblin"].GetControllerId(); got != "p-player" {
		t.Fatalf("controller_id = %q, want it filled from the now-non-empty set", got)
	}
}

func TestGrantIsIdempotent(t *testing.T) {
	// Granting twice must not duplicate. A duplicated id would make a later
	// single revoke leave a stale copy behind, so the participant would keep
	// control they were explicitly removed from.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		grantEnv(2, "thorn", "p-second"),
		grantEnv(3, "thorn", "p-second"),
	)
	assertMirror(t, st, "thorn")
	if ids := st.Actors["thorn"].GetControllerIds(); len(ids) != 2 {
		t.Fatalf("want no duplicate, got %v", ids)
	}
}

func TestRevokingTheFirstControllerPromotesTheNext(t *testing.T) {
	// controller_id tracks the SET, not the original grant: once p-player is
	// gone, an old reader must see p-second — not a departed participant.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		grantEnv(2, "thorn", "p-second"),
		revokeEnv(3, "thorn", "p-player"),
	)
	assertMirror(t, st, "thorn")
	a := st.Actors["thorn"]
	if got := a.GetControllerId(); got != "p-second" {
		t.Fatalf("controller_id = %q, want the remaining controller — a stale id here grants control to "+
			"someone who was revoked", got)
	}
	if ids := a.GetControllerIds(); len(ids) != 1 || ids[0] != "p-second" {
		t.Fatalf("want only the remaining controller, got %v", ids)
	}
}

func TestRevokingTheLastControllerEmptiesBoth(t *testing.T) {
	// Back to DM/agent-only, which is what empty has always meant.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		revokeEnv(2, "thorn", "p-player"),
	)
	assertMirror(t, st, "thorn")
	a := st.Actors["thorn"]
	if got := a.GetControllerId(); got != "" {
		t.Fatalf("controller_id = %q, want empty once nobody controls it", got)
	}
	if ids := a.GetControllerIds(); len(ids) != 0 {
		t.Fatalf("want an empty set, got %v", ids)
	}
}

func TestRevokingSomeoneWhoHasNoControlIsANoOp(t *testing.T) {
	// Idempotent in the other direction. A DM revoking twice, or revoking a
	// participant who already released, must not error or disturb the set.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		revokeEnv(2, "thorn", "p-stranger"),
	)
	assertMirror(t, st, "thorn")
	if ids := st.Actors["thorn"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-player" {
		t.Fatalf("want the set untouched, got %v", ids)
	}
}

func TestGrantAndRevokeRejectAnUnknownActor(t *testing.T) {
	// Symmetric referential integrity, matching the ConditionApplied/Removed
	// adjudication: an event naming an actor that does not exist is a fold
	// error, not a silent no-op that leaves the log meaning nothing.
	for name, env := range map[string]*vttv1.Envelope{
		"grant":  grantEnv(1, "ghost", "p-player"),
		"revoke": revokeEnv(1, "ghost", "p-player"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := engine.Apply(engine.NewState(), env); err == nil {
				t.Fatal("want an error naming the unknown actor")
			}
		})
	}
}

func TestGrantAndRevokeRejectAnEmptyParticipant(t *testing.T) {
	// An empty participant id would insert "" into the set, and controller_id
	// would then mirror an empty string while the set is non-empty — the exact
	// ambiguity ("nobody controls this" vs "someone does") the mirror rule
	// exists to prevent.
	for name, env := range map[string]*vttv1.Envelope{
		"grant":  grantEnv(2, "thorn", ""),
		"revoke": revokeEnv(2, "thorn", ""),
	} {
		t.Run(name, func(t *testing.T) {
			st := foldAll(t, actorAddedControlled(1, "thorn", "p-player"))
			if err := engine.Apply(st, env); err == nil {
				t.Fatal("want an error for an empty participant id")
			}
		})
	}
}

// TestEveryFoldPathLeavesTheMirrorIntact is the invariant checked against the
// WHOLE fold rather than the control events alone.
//
// The scenarios above each assert it at their own end, which proves the
// control arms maintain it. This one asks the broader question: does any OTHER
// event touch an actor in a way that breaks it? AdventureLoaded compiles a
// batch of ActorAdded events; a future arm might set ControllerId directly.
// The mirror is only worth anything if it holds after every event, not just
// after the two that were written with it in mind.
func TestEveryFoldPathLeavesTheMirrorIntact(t *testing.T) {
	// Deliberately includes an event that MUTATES a stored actor without being
	// a control event. ResourceChanged writes through to st.Actors, so it is
	// the shape the comment above is actually worried about: an arm that
	// touches an actor and leaves the two fields disagreeing. Folding only the
	// control arms would make this test a restatement of the ones above.
	st := foldAll(t,
		actorAddedControlled(1, "thorn", "p-player"),
		actorAddedControlled(2, "goblin", ""),
		actorAddedControlled(3, "shared", "p-a"),
		grantEnv(4, "shared", "p-b"),
		grantEnv(5, "goblin", "p-c"),
		resourceChangedEnv(6, "thorn"),
		revokeEnv(7, "shared", "p-a"),
		revokeEnv(8, "thorn", "p-player"),
	)
	for id := range st.Actors {
		assertMirror(t, st, id)
	}

	// And the specific outcomes, so this is not merely "nothing contradicted
	// itself" — a fold that emptied every set would satisfy the mirror too.
	for actorID, want := range map[string]string{
		"thorn":  "",    // last controller revoked
		"goblin": "p-c", // granted after being controllerless
		"shared": "p-b", // first controller revoked, second promoted
	} {
		a, ok := st.Actors[actorID]
		if !ok {
			t.Fatalf("actor %q missing — protobuf getters are nil-safe, so without this check an "+
				"ABSENT actor would satisfy the empty-controller row", actorID)
		}
		if got := a.GetControllerId(); got != want {
			t.Fatalf("actor %q: controller_id = %q, want %q", actorID, got, want)
		}
	}
}

// TestAddActorDropsEmptyIdsFromTheControlSet pins the guard on the ONE path
// the grant/revoke checks do not cover.
//
// controlTarget rejects an empty participant, but ActorAdded copies the
// payload's set through, and internal/gateway/convert.go builds that event
// from the client's Actor verbatim. Without this filter an ActorAdded carrying
// controller_ids:[""] creates a NON-EMPTY set whose mirror is the empty string
// — indistinguishable from an unowned actor to every reader — and revoke then
// refuses to remove it, because removing it means naming an empty participant.
// Permanently wrong, in an append-only log.
func TestAddActorDropsEmptyIdsFromTheControlSet(t *testing.T) {
	env := &vttv1.Envelope{Sequence: 1, EventId: "added", Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId:       "thorn",
			ControllerIds: []string{"", "p-player", ""},
		}},
	}}
	st := engine.NewState()
	if err := engine.Apply(st, env); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertMirror(t, st, "thorn")
	if ids := st.Actors["thorn"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-player" {
		t.Fatalf("want the empty ids dropped, got %v", ids)
	}
}

// TestAddActorLetsTheSetOverrideTheDeclaredController pins the PRECEDENCE rule
// for a payload that carries both fields and contradicts itself.
//
// The set wins, deliberately. controller_id is a MIRROR, never a second source
// of truth — treating it as a fallback here would reinstate exactly the
// authority this design removes. And the two failure directions are not
// symmetric: erasing control fails closed and is recoverable by a later
// ActorControlGranted, whereas honouring a scalar the set contradicts grants
// someone a character nobody granted them.
//
// The second case is the sharp one: {controller_id:"p-a", controller_ids:[""]}
// yields an UNOWNED actor. That is the empty-id filter doing its job, not an
// accident of statement order.
//
// Reachable verbatim — gateway/convert.go passes the client's Actor through.
func TestAddActorLetsTheSetOverrideTheDeclaredController(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		set      []string
		wantIDs  []string
		wantMirr string
	}{
		{"set overrides the declared controller", "p-a", []string{"p-b"}, []string{"p-b"}, "p-b"},
		{"a set of only empty ids erases control", "p-a", []string{""}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := engine.NewState()
			env := &vttv1.Envelope{Sequence: 1, EventId: "added", Payload: &vttv1.Envelope_ActorAdded{
				ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
					ActorId:       "thorn",
					ControllerId:  tc.declared,
					ControllerIds: tc.set,
				}},
			}}
			if err := engine.Apply(st, env); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			assertMirror(t, st, "thorn")
			a, ok := st.Actors["thorn"]
			if !ok {
				t.Fatal("actor missing")
			}
			if got := a.GetControllerIds(); len(got) != len(tc.wantIDs) {
				t.Fatalf("controller_ids = %v, want %v", got, tc.wantIDs)
			} else {
				for i := range got {
					if got[i] != tc.wantIDs[i] {
						t.Fatalf("controller_ids = %v, want %v", got, tc.wantIDs)
					}
				}
			}
			if got := a.GetControllerId(); got != tc.wantMirr {
				t.Fatalf("controller_id = %q, want %q", got, tc.wantMirr)
			}
		})
	}
}
