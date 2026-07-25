package rules_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// FuzzParse asserts Parse's only contract under arbitrary input: return
// (*Expr, nil) or (nil, error), never panic (task brief). Every seed here
// is drawn from the grammar-table and parse-error tests above — valid
// productions, every documented error case, and known parser-hazard shapes
// (deep nesting, dice edge values, stray bytes, empty/whitespace input).
func FuzzParse(f *testing.F) {
	seeds := []string{
		// valid, exercising every production
		"0", "42", "@brawn", "#pool_a",
		"3 * 4", "12 / 4", "20 / 5 / 2",
		"3 + 4", "10 - 3 - 2", "2 + 3 * 4",
		"(2 + 3) * 4", "((1 + 2) * (3 + 4))",
		"1+2*3", "  1   +   2 * 3  ", "1\t+\t2",
		"floor(5)", "half(8)", "max(3, 7)", "min(9, 2, 7, 4)",
		"max(half(10), min(2, 8))",
		"1d20", "2d6", "1d20 + @brawn",
		"max(2 * 3, half(#pool_a) + 1) - @brawn",
		"(0 - 7) / 2", "half(0 - 7)",

		// documented parse-error shapes (must error, never panic)
		"", "   ", "1 +", "+ 1", "- 1", "(-1)", "1 + + 2",
		"(1 + 2", "1 + 2)", "()", "1 + 2 3", "1 + $",
		"1d", "d6", "@", "#", "double(2)", "max(1, 2",
		"max(1, 2,)", "max()", "half()", "half(1, 2)",
		"floor(1, 2)", "max(1)", "min(1)", "0d6", "1d0",

		// known parser hazards
		"((((((((((1))))))))))",
		"1/0",
		"@a@a@a", "##", "@@", "1dd6", "1d1d1",
		"max(min(max(min(1,2),3),4),5)",
		"\x00", "\xff\xfe", "１＋２", // non-ASCII digits/operators
		",", "(,)", "1,2",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		e, err := rules.Parse(src)
		if err != nil {
			if e != nil {
				t.Fatalf("Parse(%q) returned both a non-nil Expr and an error: %v", src, err)
			}
			return
		}
		if e == nil {
			t.Fatalf("Parse(%q) returned (nil, nil) — must return a non-nil Expr on success", src)
		}
		// A successfully parsed expression must also be safely evaluable
		// (or cleanly error) against empty attrs/resources/no roller —
		// Eval must not panic either, on any AST Parse can produce.
		_, _ = rules.Eval(e, nil, nil, nil)
	})
}
