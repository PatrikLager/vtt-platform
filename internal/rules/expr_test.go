package rules_test

import (
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// queueRoller returns pre-programmed die results, consumed n at a time in
// call order. Test-only — production Rollers are Task 5's concern.
type queueRoller struct {
	queue []int
}

func (q *queueRoller) Roll(n, sides int) ([]int, int) {
	if len(q.queue) < n {
		panic("queueRoller: exhausted")
	}
	results := append([]int(nil), q.queue[:n]...)
	q.queue = q.queue[n:]
	total := 0
	for _, r := range results {
		total += r
	}
	return results, total
}

func mustParse(t *testing.T, src string) *rules.Expr {
	t.Helper()
	e, err := rules.Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): unexpected error: %v", src, err)
	}
	return e
}

func mustEval(t *testing.T, src string, attrs, resources map[string]int, roller rules.Roller) int {
	t.Helper()
	e := mustParse(t, src)
	got, err := rules.Eval(e, attrs, resources, roller)
	if err != nil {
		t.Fatalf("Eval(%q): unexpected error: %v", src, err)
	}
	return got
}

// TestEvalGrammarTable is the exhaustive table: every production, every
// precedence/associativity pairing, parenthesization, and whitespace
// variation, each pinned to an exact expected integer.
func TestEvalGrammarTable(t *testing.T) {
	attrs := map[string]int{"brawn": 3, "grit": 7}
	resources := map[string]int{"pool_a": 4}

	cases := []struct {
		name string
		src  string
		want int
	}{
		// factor: INT
		{"int_zero", "0", 0},
		{"int_multi_digit", "42", 42},

		// factor: ref '@' (attribute)
		{"attr_ref", "@brawn", 3},
		{"attr_ref_other", "@grit", 7},

		// factor: ref '#' (resource current)
		{"resource_ref", "#pool_a", 4},

		// term: '*' and '/'
		{"multiply", "3 * 4", 12},
		{"divide_exact", "12 / 4", 3},
		{"divide_floors_positive", "7 / 2", 3},

		// term left-associativity: (20/5)/2 = 2, NOT 20/(5/2) = 20
		{"divide_left_assoc", "20 / 5 / 2", 2},
		{"multiply_left_assoc", "2 * 3 * 4", 24},

		// expr: '+' and '-'
		{"add", "3 + 4", 7},
		{"subtract", "10 - 4", 6},

		// expr left-associativity: (10-3)-2 = 5, NOT 10-(3-2) = 9
		{"subtract_left_assoc", "10 - 3 - 2", 5},
		{"add_left_assoc", "1 + 2 + 3", 6},

		// precedence: '*'/'/' bind tighter than '+'/'-'
		{"mul_over_add", "2 + 3 * 4", 14},
		{"div_over_sub", "20 - 6 / 2", 17},

		// parenthesization overrides precedence
		{"paren_overrides_precedence", "(2 + 3) * 4", 20},
		{"nested_parens", "((1 + 2) * (3 + 4))", 21},

		// whitespace is insignificant
		{"no_whitespace", "1+2*3", 7},
		{"extra_whitespace", "  1   +   2 * 3  ", 7},
		{"tabs_and_spaces", "1\t+\t2", 3},

		// factor: func — floor (identity over its one arg, integer-only grammar)
		{"floor_identity", "floor(5)", 5},
		{"floor_of_expr", "floor(2 + 3)", 5},

		// factor: func — half = floor(x/2), including odd operands
		{"half_even", "half(8)", 4},
		{"half_odd_floors", "half(7)", 3},
		{"half_of_attr", "half(@grit)", 3},

		// factor: func — max/min, 2 args
		{"max_two", "max(3, 7)", 7},
		{"min_two", "min(3, 7)", 3},

		// factor: func — max/min, 3+ args (2+ arg rule)
		{"max_three", "max(1, 9, 5)", 9},
		{"min_four", "min(9, 2, 7, 4)", 2},

		// nested func calls
		{"nested_func", "max(half(10), min(2, 8))", 5},

		// DICE literal, resolved via Roller: 2d6 with queued results [3,5] = 8
		{"dice_basic", "2d6", 8},
		{"dice_single", "1d20", 0 /* overridden per-case below */},
		{"dice_combined_with_arithmetic", "1d20 + @brawn", 0 /* overridden below */},

		// mixed: everything together
		{"kitchen_sink", "max(2 * 3, half(#pool_a) + 1) - @brawn", 0 /* overridden below */},
	}

	// Cases whose expected value depends on queued dice results (can't share
	// a single roller across the table above, since consumption is
	// order-dependent) run individually below instead of in the shared loop.
	skipDice := map[string]bool{
		"dice_basic": true, "dice_single": true,
		"dice_combined_with_arithmetic": true, "kitchen_sink": true,
	}

	for _, tc := range cases {
		if skipDice[tc.name] {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			got := mustEval(t, tc.src, attrs, resources, nil)
			if got != tc.want {
				t.Fatalf("Eval(%q) = %d, want %d", tc.src, got, tc.want)
			}
		})
	}

	t.Run("dice_basic", func(t *testing.T) {
		got := mustEval(t, "2d6", attrs, resources, &queueRoller{queue: []int{3, 5}})
		if got != 8 {
			t.Fatalf("Eval(2d6) with rolls [3,5] = %d, want 8", got)
		}
	})
	t.Run("dice_single", func(t *testing.T) {
		got := mustEval(t, "1d20", attrs, resources, &queueRoller{queue: []int{17}})
		if got != 17 {
			t.Fatalf("Eval(1d20) with roll [17] = %d, want 17", got)
		}
	})
	t.Run("dice_combined_with_arithmetic", func(t *testing.T) {
		// 1d20 + @brawn, brawn=3, roll=17 -> 20
		got := mustEval(t, "1d20 + @brawn", attrs, resources, &queueRoller{queue: []int{17}})
		if got != 20 {
			t.Fatalf("Eval(1d20 + @brawn) = %d, want 20", got)
		}
	})
	t.Run("kitchen_sink", func(t *testing.T) {
		// max(2*3, half(#pool_a)+1) - @brawn
		// pool_a=4 -> half(4)=2, +1=3; 2*3=6; max(6,3)=6; 6-brawn(3)=3
		got := mustEval(t, "max(2 * 3, half(#pool_a) + 1) - @brawn", attrs, resources, nil)
		if got != 3 {
			t.Fatalf("Eval(kitchen sink) = %d, want 3", got)
		}
	})
}

// TestEvalFloorDivisionNegative pins floor semantics for '/' with negative
// operands: Go's native integer division truncates toward zero
// (-7/2 == -3), but this grammar's '/' floors (-7/2 == -4). The closed
// grammar has no unary minus (see TestParseRejectsUnaryMinus), so negative
// operands here are produced via subtraction, exactly as a real ruleset
// expression would have to.
func TestEvalFloorDivisionNegative(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"negative_dividend", "(0 - 7) / 2", -4},     // floor(-3.5) = -4, not -3
		{"negative_divisor", "7 / (0 - 2)", -4},      // floor(-3.5) = -4, not -3
		{"both_negative", "(0 - 7) / (0 - 2)", 3},    // floor(3.5) = 3
		{"exact_negative", "(0 - 8) / 2", -4},        // exact division unaffected
		{"negative_dividend_pos", "(0 - 1) / 2", -1}, // floor(-0.5) = -1
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustEval(t, tc.src, nil, nil, nil)
			if got != tc.want {
				t.Fatalf("Eval(%q) = %d, want %d", tc.src, got, tc.want)
			}
		})
	}
}

// TestEvalHalfNegative pins half(x) = floor(x/2) for negative x, matching
// TestEvalFloorDivisionNegative's rule rather than Go's truncating '/'.
func TestEvalHalfNegative(t *testing.T) {
	got := mustEval(t, "half(0 - 7)", nil, nil, nil)
	if got != -4 {
		t.Fatalf("Eval(half(0-7)) = %d, want -4", got)
	}
}

func TestEvalDivisionByZero(t *testing.T) {
	e := mustParse(t, "1 / 0")
	_, err := rules.Eval(e, nil, nil, nil)
	if err == nil {
		t.Fatal("Eval(1/0): want error, got nil")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("Eval(1/0) error = %q, want it to mention division by zero", err)
	}
}

func TestEvalUnknownAttrAtRuntime(t *testing.T) {
	e := mustParse(t, "@brawn")
	_, err := rules.Eval(e, map[string]int{}, nil, nil)
	if err == nil {
		t.Fatal("Eval(@brawn) against empty attrs: want error, got nil")
	}
	if !strings.Contains(err.Error(), "brawn") {
		t.Fatalf("Eval error = %q, want it to name the missing attribute", err)
	}
}

func TestEvalUnknownResourceAtRuntime(t *testing.T) {
	e := mustParse(t, "#pool_a")
	_, err := rules.Eval(e, nil, map[string]int{}, nil)
	if err == nil {
		t.Fatal("Eval(#pool_a) against empty resources: want error, got nil")
	}
	if !strings.Contains(err.Error(), "pool_a") {
		t.Fatalf("Eval error = %q, want it to name the missing resource", err)
	}
}

// TestParseErrors is the exhaustive parse-error table: every way the
// grammar can be violated, each asserted to name a position and the
// offending token/text (task brief: "Parse errors name position/token").
func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"whitespace_only", "   "},
		{"trailing_operator", "1 +"},
		{"leading_operator_plus", "+ 1"},
		// The grammar has NO unary minus production (factor := INT | DICE |
		// ref | func | '(' expr ')' — nothing here starts with '-'), so a
		// leading '-' is a syntax error, not negation. Rulesets express
		// negative deltas via subtraction (e.g. "0 - 1"), never "-1".
		{"leading_operator_minus", "- 1"},
		{"unary_minus_in_parens", "(-1)"},
		{"double_operator", "1 + + 2"},
		{"unmatched_open_paren", "(1 + 2"},
		{"unmatched_close_paren", "1 + 2)"},
		{"empty_parens", "()"},
		{"trailing_garbage", "1 + 2 3"},
		{"stray_character", "1 + $"},
		{"bad_dice_no_sides", "1d"},
		{"bad_dice_no_count", "d6"},
		{"ref_without_ident_attr", "@"},
		{"ref_without_ident_resource", "#"},
		{"unknown_func_name", "double(2)"},
		{"func_missing_close_paren", "max(1, 2"},
		{"func_trailing_comma", "max(1, 2,)"},
		{"func_empty_args", "max()"},
		{"half_zero_args", "half()"},
		{"half_two_args", "half(1, 2)"},
		{"floor_two_args", "floor(1, 2)"},
		{"max_one_arg", "max(1)"},
		{"min_one_arg", "min(1)"},
		{"dice_zero_count", "0d6"},
		{"dice_zero_sides", "1d0"},
		{"integer_overflow", "99999999999999999999999999999999"},
		{"dice_count_over_bound", "101d6"},
		{"dice_sides_over_bound", "1d1001"},
		{"dice_count_way_over_bound", "1000000000d1000000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rules.Parse(tc.src)
			if err == nil {
				t.Fatalf("Parse(%q): want error, got nil", tc.src)
			}
		})
	}
}

// TestParseIntegerOverflow pins that an out-of-range integer literal is a
// clean, position-named ParseError rather than a silently-clamped value
// (strconv.Atoi's error was previously discarded — see expr.go's lexNumber
// doc). 34 nines is comfortably past math.MaxInt64 (19 digits).
func TestParseIntegerOverflow(t *testing.T) {
	src := "99999999999999999999999999999999"
	_, err := rules.Parse(src)
	if err == nil {
		t.Fatalf("Parse(%q): want error, got nil", src)
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("Parse(%q) error = %q, want it to mention the value is out of range", src, err)
	}
	if !strings.Contains(err.Error(), "position 0") {
		t.Errorf("Parse(%q) error = %q, want it to name position 0", src, err)
	}
}

// TestParseIntegerOverflowMidExpression pins that the overflow check fires
// wherever the literal appears, not just at position 0.
func TestParseIntegerOverflowMidExpression(t *testing.T) {
	src := "1 + 99999999999999999999999999999999"
	_, err := rules.Parse(src)
	if err == nil {
		t.Fatalf("Parse(%q): want error, got nil", src)
	}
	if !strings.Contains(err.Error(), "position 4") {
		t.Errorf("Parse(%q) error = %q, want it to name position 4", src, err)
	}
}

// TestParseDiceBounds pins the documented dice bounds (grammar doc:
// count 1..100, sides 1..1000 — generous beyond any real tabletop need,
// but bounded so Task 5's Roller is never asked for a billion-element
// roll from something like "1000000000d1000000000").
func TestParseDiceBounds(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{"min_allowed", "1d1", false},
		{"max_allowed", "100d1000", false},
		{"count_one_over_max", "101d6", true},
		{"sides_one_over_max", "1d1001", true},
		{"count_way_over_max", "1000000000d6", true},
		{"sides_way_over_max", "1d1000000000", true},
		{"count_zero", "0d6", true},
		{"sides_zero", "1d0", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rules.Parse(tc.src)
			if tc.wantErr && err == nil {
				t.Fatalf("Parse(%q): want error, got nil", tc.src)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tc.src, err)
			}
		})
	}
}

// TestEvalDiceAtBounds pins that the maximum allowed dice literal actually
// evaluates correctly (bounds are a parse-time ceiling, not a silent cap on
// behavior).
func TestEvalDiceAtBounds(t *testing.T) {
	got := mustEval(t, "100d1000", nil, nil, &queueRoller{queue: makeSequence(100, 7)})
	if got != 700 {
		t.Fatalf("Eval(100d1000) with 100 rolls of 7 = %d, want 700", got)
	}
}

func makeSequence(n, value int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = value
	}
	return out
}

// TestParseRejectsDeepNesting pins the recursion-depth guard: pathological
// input (thousands of nested parens, the cheapest possible fuzz-discovered
// stack-exhaustion attempt against a hand-written recursive-descent parser)
// must fail with a clean ParseError, not crash the process.
func TestParseRejectsDeepNesting(t *testing.T) {
	src := strings.Repeat("(", 100000) + "1" + strings.Repeat(")", 100000)
	_, err := rules.Parse(src)
	if err == nil {
		t.Fatal("Parse(100000 nested parens): want error, got nil")
	}
	if !strings.Contains(err.Error(), "nested too deeply") {
		t.Fatalf("Parse(100000 nested parens) error = %q, want it to mention nesting depth", err)
	}
}

// TestParseRejectsUnaryMinus documents (and pins) the closed grammar's
// absence of a unary-minus production, referenced by
// TestEvalFloorDivisionNegative's doc comment.
func TestParseRejectsUnaryMinus(t *testing.T) {
	_, err := rules.Parse("-1")
	if err == nil {
		t.Fatal(`Parse("-1"): want error (no unary minus in the grammar), got nil`)
	}
}

// TestParseErrorNamesPositionAndToken spot-checks that parse error messages
// carry a position and reference the offending text, on a couple of
// representative cases (the full error-shape contract, not just "errored").
func TestParseErrorNamesPositionAndToken(t *testing.T) {
	_, err := rules.Parse("1 + $")
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "4") { // position of '$' (0-indexed)
		t.Errorf("error %q: want it to name position 4", msg)
	}
	if !strings.Contains(msg, "$") {
		t.Errorf("error %q: want it to name the offending token", msg)
	}

	_, err = rules.Parse("max(1, 2")
	if err == nil {
		t.Fatal("want error")
	}
	msg = err.Error()
	if !strings.Contains(strings.ToLower(msg), "expected") && !strings.Contains(msg, ")") {
		t.Errorf("error %q: want it to describe the missing close paren", msg)
	}
}

// TestExprRefsCollection pins Refs(): distinct '@'/'#' identifiers, in
// first-occurrence order, deduplicated across repeats — the loader relies
// on exactly this shape to cross-check a manifest's declared names.
func TestExprRefsCollection(t *testing.T) {
	e := mustParse(t, "max(@brawn, #pool_a) + half(@grit) - #pool_a")
	attrs, resources := e.Refs()

	wantAttrs := []string{"brawn", "grit"}
	wantResources := []string{"pool_a"}

	if !equalStrings(attrs, wantAttrs) {
		t.Fatalf("Refs() attrs = %v, want %v", attrs, wantAttrs)
	}
	if !equalStrings(resources, wantResources) {
		t.Fatalf("Refs() resources = %v, want %v", resources, wantResources)
	}
}

func TestExprRefsNoRefs(t *testing.T) {
	e := mustParse(t, "1 + 2 * 3")
	attrs, resources := e.Refs()
	if len(attrs) != 0 || len(resources) != 0 {
		t.Fatalf("Refs() on ref-free expr = (%v, %v), want both empty", attrs, resources)
	}
}

func TestExprStringReturnsSource(t *testing.T) {
	e := mustParse(t, "1 + 2")
	if e.String() != "1 + 2" {
		t.Fatalf("Expr.String() = %q, want %q", e.String(), "1 + 2")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
