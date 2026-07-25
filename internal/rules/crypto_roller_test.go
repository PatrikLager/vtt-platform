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
