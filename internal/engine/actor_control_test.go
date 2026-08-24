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

// controllableActor creates an actor and NOTHING ELSE. It carries no controller
// because no ActorAdded may: creation makes a character, and a grant is what
// hands it to somebody (visibility spec §5.1, Patrik's ruling 2026-08-24).
// Tests that need a controlled actor fold a grantEnv after this one, which is
// the two-step every caller now performs.
func controllableActor(seq int64, actorID string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "added", Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: actorID,
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

// TestAnActorAddedCarryingAControllerIsRefused is the fold half of "control is
// conferred exactly once, by a grant" (visibility spec §5.1, Patrik's ruling
// 2026-08-24).
//
// The fold used to SEED the control set from a declared controller_id, so
// creating an actor and handing it to somebody were the same event. That is
// what made the archer leak reachable through a second door: created with a
// controller and no kind, an actor was a party member with nobody having said
// so. The gateway refuses such a COMMAND; this refuses the EVENT, which is the
// invariant rather than the manners — an ActorAdded carrying a controller is
// not a thing that can be in a log.
//
// A REFUSAL AND NOT A SILENT DROP. Ignoring the field would fold the actor
// with no controller and say nothing, which is the quiet kind of wrong: the
// writer believes they handed a character over and the log disagrees. Fail
// closed AND loudly.
func TestAnActorAddedCarryingAControllerIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor *vttv1.Actor
	}{
		{"the mirror", &vttv1.Actor{ActorId: "thorn", ControllerId: "p-player"}},
		{"the authoritative set", &vttv1.Actor{ActorId: "thorn", ControllerIds: []string{"p-player"}}},
		{"both", &vttv1.Actor{ActorId: "thorn", ControllerId: "p-player",
			ControllerIds: []string{"p-player"}}},
		// A non-empty set of nothing. It would confer no control, and it is
		// still refused: the fold's answer must not depend on whether the id
		// the writer chose happened to be usable.
		{"a declared but empty set", &vttv1.Actor{ActorId: "thorn", ControllerIds: []string{""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := engine.NewState()
			err := engine.Apply(st, &vttv1.Envelope{Sequence: 1, EventId: "added",
				Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: tc.actor}}})
			if err == nil {
				t.Fatal("an ActorAdded that hands the actor to somebody must be refused: " +
					"creation makes a character, a grant gives it a controller")
			}
			if _, exists := st.Actors["thorn"]; exists {
				t.Error("the refusal must leave NOTHING behind — a half-created actor is worse " +
					"than a created one")
			}
		})
	}
}

func TestAddActorWithNoControllerHasAnEmptySet(t *testing.T) {
	// Empty keeps its original meaning: DM/agent only. This is the case every
	// NPC and monster is in, so it is the common one.
	st := foldAll(t, controllableActor(1, "goblin"))
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
	// A CONTROLLER IS A SECOND EVENT NOW, so `holder` names the participant a
	// kindless grant hands the actor to — the only way an actor can be
	// controlled at all — and "" means nobody. The grant is deliberately
	// KINDLESS in these rows: it must not invent a kind the log never stated,
	// which is the same claim about the grant arm that the rows make about the
	// ActorAdded arm.
	for _, tc := range []struct {
		name   string
		holder string
		kind   vttv1.ActorKind
		want   vttv1.ActorKind
	}{
		{"declared party member", "p-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"declared non-party", "", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		// A monster somebody holds: the leak §5.1 closes, and the case that
		// proves the fold does not quietly re-derive kind from control.
		{"declared non-party but controlled", "dm-1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		// Both silent shapes come back UNSPECIFIED: the log said nothing, so
		// the state says nothing. The reader (gateway.isPartyMember) then
		// treats that as NOT a party member, always — there is no longer a
		// second branch that reads control instead.
		{"undeclared and controlled", "p-player", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED},
		{"undeclared and uncontrolled", "", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envs := []*vttv1.Envelope{{Sequence: 1, EventId: "added",
				Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
					Actor: &vttv1.Actor{ActorId: "a", Kind: tc.kind}}}}}
			if tc.holder != "" {
				envs = append(envs, grantEnv(2, "a", tc.holder))
			}
			st := foldAll(t, envs...)
			if got := st.Actors["a"].GetKind(); got != tc.want {
				t.Fatalf("want the fold to store %v verbatim, got %v", tc.want, got)
			}
		})
	}
}

// grantKindEnv is grantEnv's sibling for the world after visibility spec
// §5.1's revision: a grant DECLARES what the actor is. Kept separate rather
// than adding a parameter to grantEnv, because grantEnv's kindless shape is
// exactly what every grant already recorded looks like and the tests below
// need both shapes side by side.
func grantKindEnv(seq int64, actorID, participantID string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, EventId: "grant", Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actorID, ParticipantId: participantID, Kind: kind},
	}}
}

func TestTheGrantDeclaresTheActorsKind(t *testing.T) {
	// THE REVISION, as a test (visibility spec §5.1, revised 2026-08-23).
	// Kind is not a fact about a character; it is a fact about that
	// character's STANDING RIGHT NOW, so the event that changes standing is
	// what declares it. A charmed monster becomes a player's to run and then
	// becomes a monster again — a transition, and a transition belongs on the
	// event that makes it.
	//
	// The third row is the precedence rule, and it is the one worth stating
	// out loud: a grant OVERWRITES whatever ActorAdded declared, because the
	// later event states the newer fact. That is not a special rule, it is
	// the fold's ordinary semantics; any other precedence would freeze an
	// actor's standing at creation, which is the design this revision
	// replaced.
	for _, tc := range []struct {
		name    string
		created vttv1.ActorKind
		granted vttv1.ActorKind
		want    vttv1.ActorKind
	}{
		{"an undeclared actor granted as a party member", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED,
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"an undeclared actor an agent takes to run", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED,
			vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		{"a monster charmed into a player's hands", vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		{"a party member handed to the agent that runs the table", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := foldAll(t,
				&vttv1.Envelope{Sequence: 1, EventId: "added",
					Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
						Actor: &vttv1.Actor{ActorId: "a", Kind: tc.created}}}},
				grantKindEnv(2, "a", "p-holder", tc.granted),
			)
			if got := st.Actors["a"].GetKind(); got != tc.want {
				t.Fatalf("want the grant's %v to stand, got %v", tc.want, got)
			}
		})
	}
}

func TestAKindlessGrantDoesNotEraseAKindAlreadyDeclared(t *testing.T) {
	// A GRANT THAT SAYS NOTHING SAYS NOTHING — it does not say UNSPECIFIED.
	//
	// The fold accepts a kindless grant; the refusal for one being ISSUED lives
	// at the command boundary (internal/gateway's validateGrantActorControl),
	// so this is the shape a hand-built envelope can still reach. Writing the
	// grant's kind UNCONDITIONALLY is the plausible-looking wrong thing here,
	// and it is a SILENT DEMOTION: since §5.1's migration rule was deleted
	// (2026-08-24) an absent kind is not a party member, so a declared kind
	// reset to UNSPECIFIED by a re-grant that said nothing takes the character
	// off its own party's roster — and leaves a monster that nobody declared,
	// which is the same information loss pointing the other way.
	st := foldAll(t,
		&vttv1.Envelope{Sequence: 1, EventId: "added",
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
				Actor: &vttv1.Actor{ActorId: "archer", Name: "Goblin Archer",
					Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}}},
		grantEnv(2, "archer", "p-holder"), // kindless: what an old log looks like
	)
	if got := st.Actors["archer"].GetKind(); got != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Fatalf("kind = %v, want the declaration to survive a grant that stated nothing — "+
			"UNSPECIFIED plus a controller reads as a party member, which is the leak", got)
	}
}

func TestARegrantToAnExistingControllerStillRestatesTheKind(t *testing.T) {
	// The grant's idempotency is about the CONTROL SET, never about kind.
	// Charm the goblin the agent is already running and it becomes that
	// agent's party member; the early return that stops the controller being
	// duplicated must not swallow the standing change with it — that failure
	// is silent, and it leaves a monster on the roster or a character off it.
	st := foldAll(t,
		&vttv1.Envelope{Sequence: 1, EventId: "added",
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
				Actor: &vttv1.Actor{ActorId: "archer"}}}},
		grantKindEnv(2, "archer", "p-agent", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		grantKindEnv(3, "archer", "p-agent", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
	)
	a := st.Actors["archer"]
	if got := a.GetKind(); got != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Fatalf("kind = %v, want the second grant's word to stand", got)
	}
	if ids := a.GetControllerIds(); len(ids) != 1 || ids[0] != "p-agent" {
		t.Fatalf("want the control set still un-duplicated, got %v", ids)
	}
}

func TestRevokingControlLeavesAPartyMemberAPartyMember(t *testing.T) {
	// KIND SURVIVES REVOCATION (visibility spec §5.1's second rule). A player
	// leaving the table does not turn their character into a monster:
	// revocation reassigns control, it does not restate what a character IS.
	//
	// Asserted explicitly because "revoke tidies up after itself" is the
	// plausible-looking wrong thing to write, and because its consequence is
	// invisible at the revoke — the character simply stops being on the
	// party's roster the next time they turn a corner.
	st := foldAll(t,
		&vttv1.Envelope{Sequence: 1, EventId: "added",
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
				Actor: &vttv1.Actor{ActorId: "thorn", Name: "Thorn"}}}},
		grantKindEnv(2, "thorn", "p-player", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		revokeEnv(3, "thorn", "p-player"),
	)
	a := st.Actors["thorn"]
	if got := a.GetKind(); got != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Fatalf("kind = %v, want a departed player's character to still be a party member", got)
	}
	if ids := a.GetControllerIds(); len(ids) != 0 {
		t.Fatalf("fixture check: control must actually have been given up, got %v", ids)
	}
}

func TestGrantingASecondControllerKeepsTheFirstInControllerId(t *testing.T) {
	// THE case the rejected rule got wrong. p-player must still see Thorn as
	// theirs after someone else is added.
	st := foldAll(t,
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		grantEnv(3, "thorn", "p-second"),
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
		controllableActor(1, "goblin"),
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
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		grantEnv(3, "thorn", "p-second"),
		grantEnv(4, "thorn", "p-second"),
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
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		grantEnv(3, "thorn", "p-second"),
		revokeEnv(4, "thorn", "p-player"),
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
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		revokeEnv(3, "thorn", "p-player"),
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
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		revokeEnv(3, "thorn", "p-stranger"),
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
		"grant":  grantEnv(3, "thorn", ""),
		"revoke": revokeEnv(3, "thorn", ""),
	} {
		t.Run(name, func(t *testing.T) {
			st := foldAll(t, controllableActor(1, "thorn"), grantEnv(2, "thorn", "p-player"))
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
		controllableActor(1, "thorn"),
		grantEnv(2, "thorn", "p-player"),
		controllableActor(3, "goblin"),
		controllableActor(4, "shared"),
		grantEnv(5, "shared", "p-a"),
		grantEnv(6, "shared", "p-b"),
		grantEnv(7, "goblin", "p-c"),
		resourceChangedEnv(8, "thorn"),
		revokeEnv(9, "shared", "p-a"),
		revokeEnv(10, "thorn", "p-player"),
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

// The two tests that used to sit here are DELETED, not moved, and it is worth
// saying which and why rather than leaving a gap:
//
//   - TestAddActorDropsEmptyIdsFromTheControlSet pinned that an ActorAdded
//     carrying controller_ids:[""] had its empty ids stripped.
//   - TestAddActorLetsTheSetOverrideTheDeclaredController pinned that when the
//     payload declared BOTH fields and they disagreed, the set won.
//
// Both described how the fold RECONCILED a controller declared at creation,
// and no ActorAdded may declare one any more — the arm refuses the whole shape,
// including a declared-but-empty set, which is the case the first of the two
// existed for. TestAnActorAddedCarryingAControllerIsRefused (top of this file)
// covers every input either test fed in, and asserts the stronger thing:
// nothing is stored at all.
