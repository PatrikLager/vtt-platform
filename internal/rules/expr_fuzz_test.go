package rules_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// FuzzParse asserts Parse's only contract under arbitrary input: return
// (*Expr, nil) or (nil, error), never panic (task brief). Every seed here
// is drawn from the grammar-table and parse-error tests above (v1) plus
// Task 1's new productions and their lexer-hazard edge cases (scoped refs,
// expression-sized dice, the 'd'-vs-identifier ambiguity) — valid
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

		// Task 1: scoped refs — valid
		"@caster.vim", "@target.vim", "#caster.reserve", "#target.vigor",
		"@caster . vim", "@caster.vim + @target.vim",
		"max(@caster.vim, @target.vim)", "(@caster.vim)",
		"@caster", "#target", // ident named "caster"/"target", no dot: bare ref

		// Task 1: scoped refs — error shapes (must error, never panic)
		"@self.x", "@foo.x", "#enemy.vigor", "@Caster.vim",
		"@caster.", "#target.", "@caster.)", "@caster.1",
		"@.x", "@caster..x", "@caster.vim.vim", "#.y",
		"@caster.vim.", "@1.x",

		// Task 1: expression-sized dice — valid
		"(@caster.weapon_count)d(@caster.weapon_die)",
		"1d(@caster.weapon_die)", "(1+1)d6", "1 d 6", "(2)d(6)",
		"max(1,2)d6", "1d max(6,8)", "(@n)d(@m)",
		"@caster.n d @caster.m", "1d(2d6)", "(1d1)d1",

		// Task 1: 'd'-vs-identifier lexer-hazard edge cases
		"@d6", "#d6", "@d", "#d100", "@d + 1", "@d6 + @d100",
		"d6", "d", "dice", "d100",
		"10dice", "1dice", "1d(", "1d)", "(1)d", "1d@", "1d#",
		"1d 2 d 3", "1 d(1 d 1)", "()d6", "1d()",
		"@caster.weapon_count d @caster.weapon_die",

		// Task 1: dice bounds via the new path
		"(1+1)d1001", "(101)d6", "(1+1)d0", "(0)d6",
		"(@x)d6", "1d(@x)",

		// P10 task-1 review, item 3 (controller ruling): unparenthesized
		// dice chains are now REJECTED — both the fused-token and the
		// parser-recursion shape, plus mixed fused/spaced permutations —
		// while the parenthesized forms that disambiguate them stay legal.
		"2d3d4", "2 d 3 d 4", "2d3 d4", "2 d 3d4",
		"(2d3)d4", "2d(3d4)", "1 d 2d6", "1d1d1d1",
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
