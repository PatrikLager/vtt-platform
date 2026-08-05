package rules_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Compile-time proof CryptoRoller satisfies the frozen Roller interface
// (expr.go) — a signature drift breaks the build here, matching the same
// compile-time-proof pattern harness.Conn/*Client already use elsewhere in
// this codebase.
var _ rules.Roller = (*rules.CryptoRoller)(nil)

// TestCryptoRollerResultsWithinBoundsAndSumToTotal is CryptoRoller's
// boundary-behavior pin (ADR-009): every individual result must land in
// [1, sides], there must be exactly n of them, and their sum must equal
// the returned total — checked across a range of (n, sides) pairs that
// exercise the grammar's own DICE bounds (expr.go: count 1..100, sides
// 1..1000) at both ends and in the middle, so a broken modulus/off-by-one
// in the crypto/rand draw would be caught regardless of which shape of die
// triggered it.
func TestCryptoRollerResultsWithinBoundsAndSumToTotal(t *testing.T) {
	roller := rules.NewCryptoRoller()
	cases := []struct{ n, sides int }{
		{1, 1},
		{1, 20},
		{4, 6},
		{100, 1000},
	}
	for _, tc := range cases {
		results, total := roller.Roll(tc.n, tc.sides)
		if len(results) != tc.n {
			t.Fatalf("Roll(%d, %d): got %d results, want %d", tc.n, tc.sides, len(results), tc.n)
		}
		sum := 0
		for i, r := range results {
			if r < 1 || r > tc.sides {
				t.Fatalf("Roll(%d, %d): result[%d] = %d, want in [1, %d]", tc.n, tc.sides, i, r, tc.sides)
			}
			sum += r
		}
		if sum != total {
			t.Fatalf("Roll(%d, %d): sum(results) = %d, want total = %d", tc.n, tc.sides, sum, total)
		}
	}
}

// TestCryptoRollerSingleSidedDieAlwaysOne pins the sides=1 boundary: every
// result must be exactly 1 (a d1 has only one face), not the degenerate
// modulus-by-zero result the sides<=0 guard exists to avoid.
func TestCryptoRollerSingleSidedDieAlwaysOne(t *testing.T) {
	roller := rules.NewCryptoRoller()
	results, total := roller.Roll(5, 1)
	if total != 5 {
		t.Fatalf("Roll(5, 1): total = %d, want 5", total)
	}
	for i, r := range results {
		if r != 1 {
			t.Fatalf("Roll(5, 1): result[%d] = %d, want 1", i, r)
		}
	}
}

// TestCryptoRollerZeroDiceReturnsEmpty pins the n=0 boundary: no results,
// zero total, no panic.
func TestCryptoRollerZeroDiceReturnsEmpty(t *testing.T) {
	roller := rules.NewCryptoRoller()
	results, total := roller.Roll(0, 20)
	if len(results) != 0 || total != 0 {
		t.Fatalf("Roll(0, 20) = (%v, %d), want ([], 0)", results, total)
	}
}

// TestCryptoRollerZeroSidesYieldsOneRatherThanPanicking pins the `sides > 0`
// guard, whose CONDITIONALS_BOUNDARY and CONDITIONALS_NEGATION mutants both
// survived the whole suite.
//
// Both mutants make a zero-sided die reach `rand.Int(rand.Reader,
// big.NewInt(0))`, and crypto/rand.Int PANICS for a non-positive max — so the
// guard is the only thing standing between a degenerate die and a crash inside
// a roller that has no error return to report through (see the type's doc
// comment). The existing bounds test samples four (n, sides) pairs -- {1,1},
// {1,20}, {4,6}, {100,1000} -- and none of them is 0, so the guard had never
// been exercised.
//
// WHO CAN ACTUALLY REACH sides == 0: a DIRECT caller, not Resolve. expr.go
// bounds-checks the EVALUATED sides against minDiceSides(1) immediately before
// calling Roll, so even `1d(@caster.faces)` with faces==0 -- the one form that
// could plausibly slip past the grammar -- is rejected first. Both production
// call sites (expr.go and resolve.go's recordingRoller wrapper) are on that
// path.
//
// The test earns its place anyway: Roll's own doc comment states this as a
// PROMISE -- "treats sides <= 0 as a degenerate always-1 die ... so a caller
// that somehow bypasses the parser still gets a harmless, deterministic answer
// instead of a crash" -- and Roller, CryptoRoller and NewCryptoRoller are all
// exported and wired live in internal/gateway. Pinning a documented
// exported-API contract is boundary behaviour, not an accident being
// cemented.
func TestCryptoRollerZeroSidesYieldsOneRatherThanPanicking(t *testing.T) {
	results, total := rules.CryptoRoller{}.Roll(3, 0)

	if len(results) != 3 {
		t.Fatalf("Roll(3, 0) returned %d results, want 3", len(results))
	}
	for i, r := range results {
		if r != 1 {
			t.Errorf("Roll(3, 0)[%d] = %d, want 1 — a zero-sided die yields 1, it does not roll", i, r)
		}
	}
	if total != 3 {
		t.Errorf("Roll(3, 0) total = %d, want 3", total)
	}
}

// TestCryptoRollerActuallyRolls closes a gap the rest of this file cannot see:
// nothing here could distinguish CryptoRoller from a roller that always
// returns 1.
//
// Proven, not supposed: replacing the guard with `if sides > 1000000` makes
// every die return 1 and never consults crypto/rand, and the ENTIRE suite
// stays green. The bounds test accepts 1 (it is within [1, sides]), sum-equals
// -total holds trivially, the d1 case wants 1, and the zero-sides case above
// wants 1. That is also why the `sides <= 0` NEGATION mutant survived at HEAD:
// its only observable was the panic, not the missing randomness.
//
// Forty draws of a d20 all landing on the same face has probability 20^-39.
// This is a statistical assertion and it is the honest kind: the failure it
// guards against is a roller that has stopped rolling entirely, which shows up
// on the first draw, not a subtle distribution flaw.
func TestCryptoRollerActuallyRolls(t *testing.T) {
	const draws, sides = 40, 20
	seen := map[int]bool{}
	for i := 0; i < draws; i++ {
		results, _ := rules.CryptoRoller{}.Roll(1, sides)
		if len(results) != 1 {
			t.Fatalf("Roll(1, %d) returned %d results, want 1", sides, len(results))
		}
		if results[0] < 1 || results[0] > sides {
			t.Fatalf("Roll(1, %d) = %d, outside [1, %d]", sides, results[0], sides)
		}
		seen[results[0]] = true
	}
	if len(seen) < 2 {
		t.Errorf("%d draws of a d%d produced only the value %v — this roller is not rolling",
			draws, sides, seen)
	}
}
