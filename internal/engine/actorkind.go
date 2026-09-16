package engine

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// MOVED HERE FROM internal/gateway ON 2026-09-16, by sub-project 14's Task 2,
// and the move is the point rather than a tidy-up. The per-character fan-out
// needs this answer on the write path, and neither the package that decides
// perception (internal/perceive, per that plan's D2) nor internal/campaign may
// import gateway. The choices were to duplicate the predicate or to move it
// somewhere every caller already reaches. gateway still asks the same question
// through the same function; only its address changed.
//
// ONE OTHER DEFINITION SURVIVES ON PURPOSE AND MUST NOT BE CONSOLIDATED INTO
// THIS ONE: keystone_test.go's oracleIsPartyMember. That is the keystone's
// ORACLE, and an oracle that calls the function under test measures nothing —
// its own comment records the time the arm stayed in the oracle after
// production lost it and the corpus stayed green either way. So the invariant
// is not a count: ONE function decides this for production, and the oracle is
// deliberately not it. Finding a second definition is not a defect to fix
// until you have checked which one it is.

// IsPartyMember answers the ONE question the VISIBILITY spec's §5 roster exception,
// §3.1.1's perch and the spectator's eyes all ask: is this actor one the party
// always knows about, whoever happens to hold it right now?
//
// IT LIVES IN ONE FUNCTION BECAUSE IT USED NOT TO. The same predicate was
// transcribed FIVE times: three production call sites in two files, plus TWICE
// into the keystone's oracle (visibleState and oracleEyes), which is why the
// keystone agreed with the projection while both were wrong. Counted rather
// than estimated — an earlier draft of this sentence said "a fourth time",
// having looked at one of the oracle's two copies and not the other, which is
// the same reading error the transcription itself is. Read it from here; do not
// spell it out again.
//
// KIND, NEVER THE CONTROLLER (visibility spec §5.1). Control is transient and what a
// creature IS is not. `len(GetControllerIds()) > 0`, which this replaces,
// published a monster's whole stat block to the party the moment a DM took
// control of it — and the obvious repair, asking whether a CONTROLLER is a
// player, gets both ordinary cases backwards: it drops a party member whose
// player is offline and promotes a charmed monster handed to a player. It would
// also make the projection reach into the identity store per actor per event.
//
// AN ABSENT KIND IS NOT A PARTY MEMBER. ALWAYS. One rule, no second branch,
// and nothing here reads controller_ids at all.
//
// There used to be a migration arm — absent + a controller meant party member —
// and it existed for one reason: to keep logs written before the field existed
// behaving as they had. DELETED 2026-08-24, Patrik's ruling, on the ground that
// there are no such logs: no campaign exists outside this repo's own fixtures,
// so the rule was protecting nothing while costing everything.
//
// AND THE COST WAS THE BUG. That arm is what could not tell "a log written
// before kind existed" from "a grant issued today that forgot", and every leak
// this arc chased lived in exactly that gap: control silently promoting a
// monster, an add_actor conferring party membership nobody had declared. With
// the arm gone the ambiguity has nowhere left to live, and the command-boundary
// refusals (validateGrantActorControl, validateAddActor) become belt and braces
// rather than the only thing standing between the table and the archer.
//
// So this is now a plain equality, and the fail-closed direction is total:
// every value the enum grows later — a neutral, a familiar, a summon — and
// UNSPECIFIED itself are all "not a party member" until something deliberately
// says otherwise. Visibility spec §4.4's direction: a player losing a sighting
// is a bug, a player gaining one is the defect this arc exists to prevent.
func IsPartyMember(a *vttv1.Actor) bool {
	return a.GetKind() == vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
}
