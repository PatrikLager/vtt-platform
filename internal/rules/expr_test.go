package rules_test

import (
	"errors"
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

// ============================================================================
// Task 1 (format-v2 composition, sub-project 5c): scoped references and
// expression-sized dice. Grammar and behavior per spec §5; shared interface
// signatures (Scope, EvalContext, EvalScoped, Scopes) are ADR-009 binding.
// v1's existing tables above are the byte-identical regression net for
// everything already pinned — nothing below may change their outcome.
// ============================================================================

// mustEvalScoped is EvalScoped's counterpart to mustEval above.
func mustEvalScoped(t *testing.T, src string, caster, target rules.EvalContext, roller rules.Roller) int {
	t.Helper()
	e := mustParse(t, src)
	got, err := rules.EvalScoped(e, caster, target, roller)
	if err != nil {
		t.Fatalf("EvalScoped(%q): unexpected error: %v", src, err)
	}
	return got
}

// constRoller always returns n copies of val (total = n*val). Deterministic
// and shape-agnostic (unlike queueRoller, it never runs out) — used to pin
// expression-sized dice and 'd'-operator precedence without caring about
// individual die faces.
type constRoller struct{ val int }

func (c constRoller) Roll(n, sides int) ([]int, int) {
	results := make([]int, n)
	for i := range results {
		results[i] = c.val
	}
	return results, n * c.val
}

func equalScopes(a, b []rules.Scope) bool {
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

// TestParseScopedRefsValid pins every valid shape of the extended
// ref := ('@'|'#') (scope '.')? IDENT production: scoped attribute, scoped
// resource, bare (unscoped) forms completely unaffected, a bare ref whose
// NAME happens to be "caster" (no dot: not a scope, just an identifier),
// whitespace insignificance carrying over to the new '.' token, and scoped
// refs composed into arithmetic/func-call positions exactly like bare refs
// always could be.
func TestParseScopedRefsValid(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"scoped_attr_caster", "@caster.vim"},
		{"scoped_attr_target", "@target.vim"},
		{"scoped_resource_caster", "#caster.reserve"},
		{"scoped_resource_target", "#target.vigor"},
		{"bare_attr_unaffected", "@brawn"},
		{"bare_resource_unaffected", "#pool_a"},
		{"ident_named_caster_without_dot_is_bare", "@caster"},
		{"ident_named_target_without_dot_is_bare", "#target"},
		{"whitespace_around_dot", "@caster . vim"},
		{"scoped_ref_in_arithmetic", "@caster.vim + @target.vim"},
		{"scoped_ref_in_func", "max(@caster.vim, @target.vim)"},
		{"scoped_ref_nested_in_parens", "(@caster.vim)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rules.Parse(tc.src); err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tc.src, err)
			}
		})
	}
}

// TestParseScopedRefsInvalidScope pins the exact error contract (task
// brief): only 'caster'/'target' are valid scope words — anything else
// before a '.' is a ParseError naming the position of the offending word
// ("@self.x or @foo.x = parse error naming position").
func TestParseScopedRefsInvalidScope(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantPos int // position of the offending scope word
	}{
		{"self_not_a_scope", "@self.x", 1},
		{"foo_not_a_scope", "@foo.x", 1},
		{"resource_bad_scope", "#enemy.vigor", 1},
		{"bad_scope_mid_expression", "1 + @self.x", 5},
		{"bad_scope_uppercase_rejected", "@Caster.vim", 1}, // scope words are case-sensitive
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rules.Parse(tc.src)
			if err == nil {
				t.Fatalf("Parse(%q): want error, got nil", tc.src)
			}
			var pe *rules.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) error = %v (%T), want a *rules.ParseError", tc.src, err, err)
			}
			if pe.Pos != tc.wantPos {
				t.Errorf("Parse(%q) error position = %d, want %d (error: %v)", tc.src, pe.Pos, tc.wantPos, err)
			}
		})
	}
}

// TestParseScopedRefsMissingIdentAfterDot pins the "expected identifier
// after '.'" error for a scope word with nothing (or punctuation) after the
// dot.
func TestParseScopedRefsMissingIdentAfterDot(t *testing.T) {
	cases := []string{"@caster.", "#target.", "@caster.)", "@caster.1"}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := rules.Parse(src); err == nil {
				t.Fatalf("Parse(%q): want error, got nil", src)
			}
		})
	}
}

// TestExprScopesIntrospection pins Scopes(): every ref's scope encountered
// during a walk of the expression (including refs nested inside dice
// count/sides operands), in the same traversal order Refs() already uses —
// the loader's (Task 2) primitive for load-time position-legality
// validation.
func TestExprScopesIntrospection(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []rules.Scope
	}{
		{"no_refs", "1 + 2 * 3", nil},
		{"bare_only", "max(@brawn, #pool_a)", []rules.Scope{rules.ScopeNone, rules.ScopeNone}},
		{"caster_and_target", "@caster.vim + @target.vim", []rules.Scope{rules.ScopeCaster, rules.ScopeTarget}},
		{"mixed_bare_and_scoped", "@caster.vim + @brawn", []rules.Scope{rules.ScopeCaster, rules.ScopeNone}},
		{"scoped_resources", "#caster.reserve - #target.vigor", []rules.Scope{rules.ScopeCaster, rules.ScopeTarget}},
		{"scoped_ref_inside_dice_operand", "1d(@caster.weapon_die)", []rules.Scope{rules.ScopeCaster}},
		{"scoped_refs_both_dice_operands", "(@caster.n)d(@target.m)", []rules.Scope{rules.ScopeCaster, rules.ScopeTarget}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := mustParse(t, tc.src)
			got := e.Scopes()
			if !equalScopes(got, tc.want) {
				t.Fatalf("Scopes(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

// TestEvalScopedResolutionMatrix is the scope-resolution matrix the brief
// calls for: caster attribute, target resource, an expression mixing both
// contexts, a literal-only expression needing no scope at all, a bare-ref
// eval error, and a named-missing-attribute error.
func TestEvalScopedResolutionMatrix(t *testing.T) {
	caster := rules.EvalContext{Attrs: map[string]int{"vim": 5}, Resources: map[string]int{"reserve": 3}}
	target := rules.EvalContext{Attrs: map[string]int{"vim": 2}, Resources: map[string]int{"vigor": 10}}

	t.Run("caster_attr", func(t *testing.T) {
		if got := mustEvalScoped(t, "@caster.vim", caster, target, nil); got != 5 {
			t.Fatalf("EvalScoped(@caster.vim) = %d, want 5", got)
		}
	})
	t.Run("target_resource", func(t *testing.T) {
		if got := mustEvalScoped(t, "#target.vigor", caster, target, nil); got != 10 {
			t.Fatalf("EvalScoped(#target.vigor) = %d, want 10", got)
		}
	})
	t.Run("mixed_expr_both_contexts", func(t *testing.T) {
		if got := mustEvalScoped(t, "@caster.vim - @target.vim", caster, target, nil); got != 3 {
			t.Fatalf("EvalScoped(@caster.vim - @target.vim) = %d, want 3", got)
		}
	})
	t.Run("literal_only_needs_no_scope", func(t *testing.T) {
		if got := mustEvalScoped(t, "2 + 3 * 4", caster, target, nil); got != 14 {
			t.Fatalf("EvalScoped(2+3*4) = %d, want 14", got)
		}
	})
	t.Run("bare_ref_is_eval_error", func(t *testing.T) {
		e := mustParse(t, "@brawn")
		if _, err := rules.EvalScoped(e, caster, target, nil); err == nil {
			t.Fatal("EvalScoped(@brawn) [bare ref]: want error, got nil")
		}
	})
	t.Run("caster_missing_attr_names_it", func(t *testing.T) {
		// P10 task-1 review, item 5 (low): assert the SCOPE word appears
		// too, not just the identifier — "unknown attribute missing" alone
		// wouldn't tell you which context was consulted.
		e := mustParse(t, "@caster.missing")
		_, err := rules.EvalScoped(e, caster, target, nil)
		if err == nil {
			t.Fatalf("EvalScoped(@caster.missing) error = %v, want an error", err)
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("EvalScoped(@caster.missing) error = %v, want it to name the missing attribute", err)
		}
		if !strings.Contains(err.Error(), "caster") {
			t.Errorf("EvalScoped(@caster.missing) error = %v, want it to name the scope (caster)", err)
		}
	})
	t.Run("target_missing_resource_names_it", func(t *testing.T) {
		e := mustParse(t, "#target.missing_pool")
		_, err := rules.EvalScoped(e, caster, target, nil)
		if err == nil {
			t.Fatalf("EvalScoped(#target.missing_pool) error = %v, want an error", err)
		}
		if !strings.Contains(err.Error(), "missing_pool") {
			t.Errorf("EvalScoped(#target.missing_pool) error = %v, want it to name the missing resource", err)
		}
		if !strings.Contains(err.Error(), "target") {
			t.Errorf("EvalScoped(#target.missing_pool) error = %v, want it to name the scope (target)", err)
		}
	})
}

// TestParseScopedRefNamedLikeAFunctionIsStillARef (P10 task-1 review, item
// 5 low) pins that a scoped ref's NAME — the identifier after '.' — is
// NEVER routed through parseFuncCall, even when it collides with a real
// function name: "@caster.max" is the attribute "max" scoped to caster,
// not an attempt to call max(). Trailing "(1,2)" is therefore unconsumed
// input (a generic parse error), never a function-arity error naming
// "max" — proving parseRef's second identifier is consumed as a plain ref
// name, not handed to parseFuncCall.
func TestParseScopedRefNamedLikeAFunctionIsStillARef(t *testing.T) {
	_, err := rules.Parse("@caster.max(1,2)")
	if err == nil {
		t.Fatal(`Parse("@caster.max(1,2)"): want error (trailing "(1,2)" after the complete ref), got nil`)
	}
	if strings.Contains(err.Error(), "expects") {
		t.Errorf(`Parse("@caster.max(1,2)") error = %q, want a trailing-input error, NOT a function-arity error (would prove "max" was misrouted to a function-call attempt)`, err.Error())
	}
}

// TestEvalScopedRefInSingleContextEvalIsError is the defense-in-depth pair
// (brief: "both ways") to the bare-ref case above: a scoped ref fed to
// single-actor Eval must also error, not silently guess a context.
func TestEvalScopedRefInSingleContextEvalIsError(t *testing.T) {
	cases := []string{"@caster.vim", "#target.vigor"}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			e := mustParse(t, src)
			_, err := rules.Eval(e, map[string]int{"vim": 1}, map[string]int{"vigor": 1}, nil)
			if err == nil {
				t.Fatalf("Eval(%q) [scoped ref]: want error, got nil", src)
			}
		})
	}
}

// TestParseExpressionSizedDice pins every new DICE shape the brief names:
// parenthesized count and sides, a literal count with parenthesized sides,
// a parenthesized count with a literal-looking (lexically ambiguous — see
// expr.go's tokenizing-'d' doc) sides, whitespace around the 'd' operator,
// and func-call expressions on either side of 'd'.
func TestParseExpressionSizedDice(t *testing.T) {
	cases := []string{
		"(@caster.weapon_count)d(@caster.weapon_die)",
		"1d(@caster.weapon_die)",
		"(1+1)d6",
		"1 d 6",
		"(2)d(6)",
		"max(1,2)d6",
		"1d max(6,8)",
		"(@n)d(@m)",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := rules.Parse(src); err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", src, err)
			}
		})
	}
}

// TestEvalDicePrecedence pins the brief's exact equivalences: 'd' binds at
// the factor level, tighter than both '*'/'/' and '+'/'-'. Every die is
// forced to the same value via constRoller so the arithmetic result alone
// discriminates between the two possible parses.
func TestEvalDicePrecedence(t *testing.T) {
	t.Run("dice_plus_binds_tighter_than_plus", func(t *testing.T) {
		// 1d6+2 == (1d6)+2: forced roll 4 -> 1d6=4, +2 = 6. If '+' bound
		// tighter, this would instead try to parse as 1d(6+2) = 1d8 = 4.
		got := mustEval(t, "1d6+2", nil, nil, constRoller{val: 4})
		want := mustEval(t, "(1d6)+2", nil, nil, constRoller{val: 4})
		if got != want || got != 6 {
			t.Fatalf("Eval(1d6+2) = %d, Eval((1d6)+2) = %d, want both 6", got, want)
		}
	})
	t.Run("dice_binds_tighter_than_multiply", func(t *testing.T) {
		// 2*3d6 == 2*(3d6): forced roll 4 -> 3d6=12, 2*12=24. If '*' bound
		// tighter, this would instead parse as (2*3)d6 = 6d6 = 24 too by
		// coincidence at this forced value, so this case ALSO asserts
		// against the literal count the two parses would each roll.
		got := mustEval(t, "2*3d6", nil, nil, constRoller{val: 4})
		want := mustEval(t, "2*(3d6)", nil, nil, constRoller{val: 4})
		if got != want || got != 24 {
			t.Fatalf("Eval(2*3d6) = %d, Eval(2*(3d6)) = %d, want both 24", got, want)
		}
	})
}

// TestEvalDicePrecedenceDiscriminates re-pins the multiply case with a
// roller/value pairing where '(2*3)d6' and '2*(3d6)' produce DIFFERENT
// totals, so the assertion can't pass by coincidence the way equal forced
// rolls might.
func TestEvalDicePrecedenceDiscriminates(t *testing.T) {
	// constRoller ignores 'sides' and returns n copies of val, so the two
	// parses differ only in n (dice count): 2*(3d6) rolls 3 dice, whereas
	// (2*3)d6 would roll 6. 3*val=6 vs 6*val=12 for val=2 — distinct.
	got := mustEval(t, "2*3d6", nil, nil, constRoller{val: 2})
	if got != 12 { // 2 * (3 * 2) = 12, NOT (6 * 2) = 12... see below
		t.Fatalf("Eval(2*3d6) = %d, want 12 (2*(3d6) with forced-2 rolls: 2*(3*2)=12)", got)
	}
	// The discriminator: total dice count actually rolled. A roller that
	// records n lets us assert the *3d6* subexpression rolled exactly 3
	// dice, not 6.
	rec := &recordingRoller{}
	if _, err := rules.Eval(mustParse(t, "2*3d6"), nil, nil, rec); err != nil {
		t.Fatalf("Eval(2*3d6): unexpected error: %v", err)
	}
	if rec.lastN != 3 {
		t.Fatalf("Eval(2*3d6) rolled %d dice, want 3 (2*(3d6), not (2*3)d6 which would roll 6)", rec.lastN)
	}
}

type recordingRoller struct{ lastN, lastSides int }

func (r *recordingRoller) Roll(n, sides int) ([]int, int) {
	r.lastN, r.lastSides = n, sides
	results := make([]int, n)
	for i := range results {
		results[i] = 1
	}
	return results, n
}

// TestEvalScopedExpressionSizedDice exercises the flagship expression-sized
// dice scenario from spec §5: weapon dice read from actor data instead of a
// platform equipment concept.
func TestEvalScopedExpressionSizedDice(t *testing.T) {
	caster := rules.EvalContext{Attrs: map[string]int{"weapon_count": 2, "weapon_die": 8}}
	target := rules.EvalContext{}
	got, err := rules.EvalScoped(mustParse(t, "(@caster.weapon_count)d(@caster.weapon_die)"), caster, target, constRoller{val: 3})
	if err != nil {
		t.Fatalf("EvalScoped: unexpected error: %v", err)
	}
	if got != 6 { // 2 dice, forced to 3 each = 6
		t.Fatalf("EvalScoped((@caster.weapon_count)d(@caster.weapon_die)) = %d, want 6", got)
	}
}

// permissiveRoller is a REAL (non-nil) Roller for TestEvalDiceEvalTimeBounds
// (P10 task-1 review, item 2 fix). The original version of this test used a
// nil Roller, so a broken/deleted bounds check would fall through to
// evalTree's "requires a non-nil Roller" guard — whose message embeds
// "%dd%d" and therefore happens to CONTAIN the very digits the test was
// asserting on (e.g. "dice 0d6 requires a non-nil Roller" contains "0"),
// making the assertion pass for the wrong reason. A real Roller removes
// that escape hatch entirely: if the bounds check is gone, Roll() actually
// executes and the test must fail. Unlike constRoller, permissiveRoller
// tolerates n<=0 without panicking (make([]int, n) panics for negative n)
// so it stays safe even when exercised against deliberately-broken code
// (see the injection-proof transcript in the fix report).
type permissiveRoller struct{ val int }

func (r permissiveRoller) Roll(n, sides int) ([]int, int) {
	if n <= 0 {
		return nil, 0
	}
	results := make([]int, n)
	for i := range results {
		results[i] = r.val
	}
	return results, n * r.val
}

// TestEvalDiceEvalTimeBounds pins non-literal count/sides bounds: deferred
// to Eval/EvalScoped time (unlike a literal 'NdM', checked at Parse time),
// with a clean error naming the computed out-of-bounds value, WHICH bound
// (count vs sides) it is, and that bound's exact range — and that the
// INCLUSIVE boundary values (100, 1000) succeed and actually roll. Every
// subtest passes permissiveRoller (never nil) — see its doc comment for
// why that is load-bearing for this test's genuineness.
func TestEvalDiceEvalTimeBounds(t *testing.T) {
	roller := permissiveRoller{val: 5}

	assertBoundsError := func(t *testing.T, err error, wantSubstrings ...string) {
		t.Helper()
		if err == nil {
			t.Fatal("want error, got nil")
		}
		msg := err.Error()
		for _, want := range wantSubstrings {
			if !strings.Contains(msg, want) {
				t.Errorf("error = %q, want it to contain %q", msg, want)
			}
		}
	}

	t.Run("computed_count_zero", func(t *testing.T) {
		e := mustParse(t, "(@count)d6")
		_, err := rules.Eval(e, map[string]int{"count": 0}, nil, roller)
		assertBoundsError(t, err, "out of bounds", "0", "count", "[1, 100]")
	})
	t.Run("computed_sides_too_large", func(t *testing.T) {
		e := mustParse(t, "1d(@sides)")
		_, err := rules.Eval(e, map[string]int{"sides": 2000}, nil, roller)
		assertBoundsError(t, err, "out of bounds", "2000", "sides", "[1, 1000]")
	})
	t.Run("computed_count_negative_via_subtraction", func(t *testing.T) {
		_, err := rules.Eval(mustParse(t, "(0-1)d6"), nil, nil, roller)
		assertBoundsError(t, err, "out of bounds", "-1", "count", "[1, 100]")
	})
	t.Run("computed_count_over_bound_via_scoped_attr", func(t *testing.T) {
		caster := rules.EvalContext{Attrs: map[string]int{"n": 101}}
		_, err := rules.EvalScoped(mustParse(t, "(@caster.n)d6"), caster, rules.EvalContext{}, roller)
		assertBoundsError(t, err, "out of bounds", "101", "count", "[1, 100]")
	})
	t.Run("computed_count_at_lower_boundary_100_succeeds_and_rolls", func(t *testing.T) {
		// The boundary is INCLUSIVE: exactly 100 (the max) must succeed —
		// not be rejected by an off-by-one — and must actually invoke the
		// roller (proven by the returned total depending on it).
		got, err := rules.Eval(mustParse(t, "(@count)d6"), map[string]int{"count": 100}, nil, roller)
		if err != nil {
			t.Fatalf("Eval((@count)d6) with count=100: unexpected error: %v", err)
		}
		if got != 100*roller.val {
			t.Fatalf("Eval((@count)d6) with count=100 = %d, want %d (100 dice actually rolled)", got, 100*roller.val)
		}
	})
	t.Run("computed_sides_at_upper_boundary_1000_succeeds_and_rolls", func(t *testing.T) {
		got, err := rules.Eval(mustParse(t, "1d(@sides)"), map[string]int{"sides": 1000}, nil, roller)
		if err != nil {
			t.Fatalf("Eval(1d(@sides)) with sides=1000: unexpected error: %v", err)
		}
		if got != roller.val {
			t.Fatalf("Eval(1d(@sides)) with sides=1000 = %d, want %d", got, roller.val)
		}
	})
}

// TestParseExpressionSizedDiceLiteralBoundsStillAtParseTime pins that a
// literal integer operand reached via the NEW factor-'d'-factor path (not
// just the old lexer-fusion fast path) still gets its bounds check at
// Parse time, exactly like v1's pure-literal 'NdM'.
func TestParseExpressionSizedDiceLiteralBoundsStillAtParseTime(t *testing.T) {
	cases := []struct {
		src  string
		want string // substring the bounds error must contain
	}{
		{"(1+1)d1001", "1001"}, // literal sides, reached via the "d"+digits split
		{"(101)d6", "101"},     // literal count, reached via a transparent paren-wrap
		{"(1+1)d0", "0"},       // literal sides zero
		{"(0)d6", "0"},         // literal count zero
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			_, err := rules.Parse(tc.src)
			if err == nil {
				t.Fatalf("Parse(%q): want a parse-time error, got nil", tc.src)
			}
			if !strings.Contains(err.Error(), "out of bounds") {
				t.Fatalf("Parse(%q) error = %q, want it to be a dice-bounds error (contains %q)", tc.src, err.Error(), tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse(%q) error = %q, want it to name the computed value %q", tc.src, err.Error(), tc.want)
			}
		})
	}
}

// TestExprHasDiceCoversExpressionSizedDice pins that HasDice() detects the
// new expression-sized productions exactly like it already detects a
// literal NdM (v1) — the loader's dice-ban positions (threshold 'when',
// default_max_expr) must reject these too.
func TestExprHasDiceCoversExpressionSizedDice(t *testing.T) {
	cases := []string{
		"(@caster.weapon_count)d(@caster.weapon_die)",
		"1d(@caster.weapon_die)",
		"(1+1)d6",
		"5 + 1d(@sides)",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if !mustParse(t, src).HasDice() {
				t.Fatalf("HasDice(%q) = false, want true", src)
			}
		})
	}
}

// TestExprRefsCollectsRefsInsideDiceOperands pins that Refs() (the loader's
// cross-reference primitive) walks INTO dice count/sides sub-expressions —
// a v2 requirement that didn't exist for v1's always-literal dice.
func TestExprRefsCollectsRefsInsideDiceOperands(t *testing.T) {
	e := mustParse(t, "1d(@sides) + #pool")
	attrs, resources := e.Refs()
	if !equalStrings(attrs, []string{"sides"}) {
		t.Fatalf("Refs() attrs = %v, want [sides]", attrs)
	}
	if !equalStrings(resources, []string{"pool"}) {
		t.Fatalf("Refs() resources = %v, want [pool]", resources)
	}
}

// TestParseBareDIdentifierStillAvailableAsRefName pins that "d"/"d6"-shaped
// text remains a completely ordinary identifier when it appears as a ref
// NAME (right after '@'/'#') — the new dice-operator detection only fires
// in the OTHER position (immediately after a complete factor), so it can
// never steal a legitimate attribute/resource name.
func TestParseBareDIdentifierStillAvailableAsRefName(t *testing.T) {
	cases := []string{"@d6", "#d6", "@d", "#d100", "@d + 1", "@d6 + @d100"}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := rules.Parse(src); err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", src, err)
			}
		})
	}
}

// TestParseBareDAloneAtStartStillErrors pins that a bare "d6" (or "d", or
// any other identifier) at the START of an expression — no preceding
// factor — is UNCHANGED from v1: still a parse error, because parsePrimary
// (formerly parseFactor's switch) requires '(' after any bare identifier
// (function-call only). The new post-primary dice-operator check never
// runs here at all, since nothing has been parsed yet to be "after".
func TestParseBareDAloneAtStartStillErrors(t *testing.T) {
	cases := []string{"d6", "d", "dice", "d100"}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := rules.Parse(src); err == nil {
				t.Fatalf("Parse(%q): want error, got nil", src)
			}
		})
	}
}

// TestParseDiceChainingHazardsDoNotPanic pins the fuzz-corpus "known
// hazards" (1dd6, 1d1d1) to a concrete outcome rather than leaving them
// solely to the fuzz target: both terminate cleanly with a ParseError,
// never panicking. 1d1d1 was ORIGINALLY documented (and tested) as
// succeeding right-associatively as 1d(1d1) — the P10 task-1 review's
// controller ruling (item 3) overturned that: an unparenthesized dice
// chain is now rejected outright (see TestParseDiceChainRejectedWithoutParens
// and expr.go's grammar doc, "Dice chaining"), and "1d1d1" is exactly that
// shape (lexer-fuses "1d1" first, then the trailing "d1" would chain onto
// it) — so it is now the SAME ambiguous-chain ParseError, not a successful
// parse.
func TestParseDiceChainingHazardsDoNotPanic(t *testing.T) {
	if _, err := rules.Parse("1dd6"); err == nil {
		t.Error(`Parse("1dd6"): want error (not a valid dice or identifier shape), got nil`)
	}
	_, err := rules.Parse("1d1d1")
	if err == nil {
		t.Fatal(`Parse("1d1d1"): want error (ambiguous unparenthesized dice chain per the controller ruling), got nil`)
	}
	if !strings.Contains(err.Error(), "ambiguous dice chain") {
		t.Errorf(`Parse("1d1d1") error = %q, want the ambiguous-dice-chain error`, err.Error())
	}
}

// ============================================================================
// P10 task-1 review fixes (controller-approved verdict). Four items:
//  1. CRITICAL — unguarded recursion in parseFactor's bare-'d' sides path
//     (fatal stack overflow on a long chain). Fixed by depth-accounting
//     parseFactor AND by item 3's ruling, which removes the recursive
//     chain-continuation path entirely.
//  2. HIGH — TestEvalDiceEvalTimeBounds verified nothing (nil Roller let a
//     deleted bounds check hide behind a coincidentally-matching fallback
//     error). Fixed above (permissiveRoller, stronger assertions,
//     boundary cases); injection-proof transcript is in the fix report.
//  3. MEDIUM-HIGH — dice-chain semantics were whitespace-sensitive
//     ('2d3d4' vs '2 d 3 d 4' meant different things). CONTROLLER RULING:
//     reject unparenthesized chains outright. Tested below.
//  4. MEDIUM — Refs()/Scopes() aren't positionally zippable. Fixed by
//     ScopedRefs() below.
// ============================================================================

// recordingSeqRoller records every Roll(n, sides) call, in order — used to
// prove EXACTLY which dice were rolled and in what sequence, discriminating
// '(2d3)d4' from '2d(3d4)' by their different call shapes (item 3).
type recordingSeqRoller struct {
	calls [][2]int
	val   int
}

func (r *recordingSeqRoller) Roll(n, sides int) ([]int, int) {
	r.calls = append(r.calls, [2]int{n, sides})
	results := make([]int, n)
	for i := range results {
		results[i] = r.val
	}
	return results, n * r.val
}

func equalCallSeqs(a, b [][2]int) bool {
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

// TestParseDiceChainRejectedWithoutParens is the controller ruling (P10
// task-1 review, item 3): a dice operand of a bare 'd' may not itself be a
// bare dice — neither a lexer-fused chain ('2d3d4', which would silently
// mean '(2d3)d4') nor a parser-recursed chain ('2 d 3 d 4', which would
// silently mean '2d(3d4)' — the SAME characters modulo whitespace meaning
// something different is exactly the implicit-contract class spec §2
// decision 4 ("no new implicit contracts") bans. Both shapes — and their
// fused/spaced permutations — are a ParseError naming the ambiguity.
func TestParseDiceChainRejectedWithoutParens(t *testing.T) {
	cases := []string{
		"2d3d4",     // fully lexer-fused: '2d3' fuses first, then chains onto 'd4'
		"2 d 3 d 4", // fully spaced: parser-recursion chain
		"2d3 d4",    // count fused, chain spaced
		"2 d 3d4",   // count spaced, sides fused
		"1d1d1",     // the original (now-overturned) "succeeds" example
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			_, err := rules.Parse(src)
			if err == nil {
				t.Fatalf("Parse(%q): want error (ambiguous unparenthesized dice chain), got nil", src)
			}
			var pe *rules.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) error = %v (%T), want a *rules.ParseError", src, err, err)
			}
			if !strings.Contains(pe.Msg, "ambiguous dice chain") {
				t.Errorf("Parse(%q) error = %q, want it to say \"ambiguous dice chain\"", src, pe.Msg)
			}
			if !strings.Contains(pe.Msg, "parenthesize") {
				t.Errorf("Parse(%q) error = %q, want it to suggest parenthesizing", src, pe.Msg)
			}
		})
	}
}

// TestEvalDiceChainParenthesizedFormsDiscriminate is the OTHER half of the
// controller ruling: explicit parens remain fully legal and disambiguate
// exactly as written. A roller that records every (n, sides) call proves
// '(2d3)d4' and '2d(3d4)' really do roll in a different order/shape, not
// just that they happen to total the same — the reviewer's own method.
func TestEvalDiceChainParenthesizedFormsDiscriminate(t *testing.T) {
	t.Run("count_side_parenthesized", func(t *testing.T) {
		r := &recordingSeqRoller{val: 5}
		got, err := rules.Eval(mustParse(t, "(2d3)d4"), nil, nil, r)
		if err != nil {
			t.Fatalf("Eval((2d3)d4): unexpected error: %v", err)
		}
		// Inner '2d3' rolls first: Roll(2,3) -> total 10 (2*5). That total
		// becomes the COUNT of the outer roll: Roll(10,4).
		want := [][2]int{{2, 3}, {10, 4}}
		if !equalCallSeqs(r.calls, want) {
			t.Fatalf("Eval((2d3)d4) roller calls = %v, want %v", r.calls, want)
		}
		if got != 10*5 { // 10 dice (the outer roll's count), each forced to 5
			t.Fatalf("Eval((2d3)d4) = %d, want %d", got, 10*5)
		}
	})
	t.Run("sides_side_parenthesized", func(t *testing.T) {
		r := &recordingSeqRoller{val: 5}
		got, err := rules.Eval(mustParse(t, "2d(3d4)"), nil, nil, r)
		if err != nil {
			t.Fatalf("Eval(2d(3d4)): unexpected error: %v", err)
		}
		// Inner '3d4' rolls first: Roll(3,4) -> total 15 (3*5). That total
		// becomes the SIDES of the outer roll: Roll(2,15).
		want := [][2]int{{3, 4}, {2, 15}}
		if !equalCallSeqs(r.calls, want) {
			t.Fatalf("Eval(2d(3d4)) roller calls = %v, want %v", r.calls, want)
		}
		if got != 2*5 { // 2 dice, each forced to 5
			t.Fatalf("Eval(2d(3d4)) = %d, want %d", got, 2*5)
		}
	})
	// The two forms are NOT interchangeable — different call sequences,
	// pinned above — this is the direct discriminator the reviewer asked
	// for: [[2 3][? 4]] vs [[3 4][2 ?]].
}

// TestParseRejectsDeepBareDiceChain is the CRITICAL fix's regression test
// (P10 task-1 review, item 1). BEFORE this fix, parseFactor's bare-'d'
// sides operand was parsed via an UNGUARDED recursive call to parseFactor
// itself (no p.depth accounting) — a long enough chain of bare 'd's
// overflowed the goroutine stack (the reviewer proved this fatally at
// ~5M repetitions). Two independent fixes now apply: (1) the controller
// ruling (item 3) rejects ANY unparenthesized dice chain outright, so this
// input is refused at the SECOND 'd' encountered — O(1) in chain length,
// not proportional to it — and (2) parseFactor is depth-accounted like
// parseExpr regardless, as a second, independent bound. 10,000 repetitions
// (not millions) is already enough to prove there is no crash and no
// chain-length-proportional cost, which keeps this test fast.
func TestParseRejectsDeepBareDiceChain(t *testing.T) {
	src := "1" + strings.Repeat(" d 1", 10000)
	_, err := rules.Parse(src)
	if err == nil {
		t.Fatal("Parse(10,000-long bare-'d' chain): want error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous dice chain") {
		t.Errorf("Parse(10,000-long bare-'d' chain) error = %q, want the ambiguous-dice-chain error (proves rejection is immediate, not depth-guard fallback)", err.Error())
	}
}

// TestExprScopedRefsPerOccurrence (P10 task-1 review, item 4 — MEDIUM):
// Refs() dedupes by name and Scopes() doesn't carry names, so the two
// cannot be zipped together to recover (name, scope) pairs. ScopedRefs()
// is the single primitive Task 2's loader needs: one entry per ref
// OCCURRENCE, in AST-walk order, carrying sigil + scope + name together —
// pinned here with a repeated name under different (and the same) scopes
// to prove it is genuinely per-occurrence, not deduplicated.
func TestExprScopedRefsPerOccurrence(t *testing.T) {
	e := mustParse(t, "@caster.vim + #target.vim + @caster.vim")
	got := e.ScopedRefs()
	want := []rules.ScopedRef{
		{Sigil: '@', Scope: rules.ScopeCaster, Name: "vim"},
		{Sigil: '#', Scope: rules.ScopeTarget, Name: "vim"},
		{Sigil: '@', Scope: rules.ScopeCaster, Name: "vim"},
	}
	if len(got) != len(want) {
		t.Fatalf("ScopedRefs() = %+v (%d entries), want %d entries", got, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ScopedRefs()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestExprScopedRefsIncludesBareAndDiceOperands rounds out ScopedRefs'
// coverage the same way Refs/Scopes are pinned: bare (ScopeNone) refs, and
// refs nested inside dice count/sides operands, both included.
func TestExprScopedRefsIncludesBareAndDiceOperands(t *testing.T) {
	e := mustParse(t, "@brawn + 1d(@caster.weapon_die)")
	got := e.ScopedRefs()
	want := []rules.ScopedRef{
		{Sigil: '@', Scope: rules.ScopeNone, Name: "brawn"},
		{Sigil: '@', Scope: rules.ScopeCaster, Name: "weapon_die"},
	}
	if len(got) != len(want) {
		t.Fatalf("ScopedRefs() = %+v (%d entries), want %d entries", got, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ScopedRefs()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestExprScopedRefsNoRefs(t *testing.T) {
	e := mustParse(t, "1 + 2 * 3")
	if got := e.ScopedRefs(); len(got) != 0 {
		t.Fatalf("ScopedRefs() on ref-free expr = %+v, want empty", got)
	}
}

// TestParseDiceBoundsAreInclusive pins every dice limit ON its limit, through
// BOTH validation paths.
//
// minDiceCount=1, maxDiceCount=100, minDiceSides=1, maxDiceSides=1000 are all
// INCLUSIVE. FOUR CONDITIONALS_BOUNDARY mutants survived here — expr.go:1036,
// :1065 and :1080 (two on that line) — all of them on the PARSER's copies of
// the checks. The lexer's copies at :861/:864 were already pinned by
// TestParseDiceBounds, 900 lines above; this test deliberately repeats those
// rows so that older test's coverage is visible next to the new work rather
// than implied.
//
// THE BOUNDS ARE CHECKED THREE TIMES, in three places, and a test using only
// the bare `NdM` form reaches just one of them. The LEXER folds `0d6` into a single
// dice token and rejects it at expr.go:862/:865. The PARSER checks again at
// :1037/:1066/:1081, for when the count or sides arrive as separate nodes —
// `(1)d6`, `1 d 6`, `1d(6)` — because a bare integer is only subject to the
// dice range when it is ACTUALLY used as a count (the same "500" is an
// ordinary integer everywhere else in the grammar). And :1080 checks a THIRD
// time for a fused "d"+digits token following a separate count — `(1)d1`,
// which neither `1d1` nor `1d(1)` reaches.
//
// A first version of this test used only bare literals, and every dice mutant
// appeared to survive it. That reading was WRONG in an instructive way: the
// injections were run against THIS TEST ALONE (`go test -run
// TestParseDiceBounds...`), while a mutant "survives" only if the WHOLE SUITE
// passes under it. The lexer mutants were already dead to TestParseDiceBounds;
// only the parser ones were ever live. The bare-literal gap was real — that
// second path had no test — but the count was an artifact of a too-narrow
// injection. Run the suite, not the test.
//
// Each case is paired: the legal extreme MUST parse, one step past it MUST
// NOT. Rejections alone would pass with a bound shifted the permissive way;
// acceptances alone would pass with it shifted the other way.
func TestParseDiceBoundsAreInclusive(t *testing.T) {
	legal := []string{
		// Lexer path: a single dice token.
		"1d1", "1d1000", "100d1", "100d1000",
		// Parser path: count and/or sides as separate nodes.
		"(1)d6", "(100)d6", "1 d 6", "1d(1)", "1d(1000)", "(100)d(1000)",
		// THIRD path: a separate count followed by a FUSED "d"+digits token,
		// which lexIdent consumes whole and expr.go:1080 re-checks on its own.
		// `1d(1)` does not reach it — the parenthesised sides take :1065.
		"(1)d1", "(1)d1000",
	}
	for _, src := range legal {
		t.Run("legal/"+src, func(t *testing.T) {
			if _, err := rules.Parse(src); err != nil {
				t.Errorf("Parse(%q) = %v, want it accepted — the dice bounds are inclusive", src, err)
			}
		})
	}

	illegal := []string{
		// Lexer path.
		"0d6", "101d6", "1d0", "1d1001",
		// Parser path — the one a bare-literal test cannot reach.
		"(0)d6", "(101)d6", "1d(0)", "1d(1001)",
		// Fused-sides path.
		"(1)d0", "(1)d1001",
	}
	for _, src := range illegal {
		t.Run("illegal/"+src, func(t *testing.T) {
			// The REASON, not just the rejection: neutering each of the five
			// bound guards showed no second guard backstops any of them today,
			// so a bare err != nil is non-vacuous — but it would stop being so
			// the moment one appeared, silently. "out of bounds" costs nothing
			// and encodes what the assertion is actually for.
			_, err := rules.Parse(src)
			if err == nil {
				t.Fatalf("Parse(%q) succeeded, want it rejected — one step outside an inclusive bound", src)
			}
			if !strings.Contains(err.Error(), "out of bounds") {
				t.Errorf("Parse(%q) = %v, want a dice BOUNDS rejection — some other guard catching this "+
					"would leave the bound itself untested", src, err)
			}
		})
	}
}

// TestParseDepthAndArityBoundsAreInclusive pins the depth guards that survived
// because nothing tested them at their limits, plus the arity minimum for
// readability.
//
//   - maxExprDepth (200) is INCLUSIVE: `p.depth > maxExprDepth` at
//     expr.go:923 and :1002. Exactly 200 must parse; 201 must not. Nothing
//     nested anywhere near that deep existed.
//   - `defer p.depth--` at :922 and :1001. Mutated to `p.depth++` the counter
//     never unwinds, so depth accumulates across SIBLINGS as well as nesting
//     and a long FLAT expression trips a limit meant only for nesting. No test
//     was long enough to notice.
//     NOTE the arity subtests below kill NOTHING new: `len(args) < arity.min`
//     at :1298:32 was already dead to the existing max(3,7)/max(1) cases, and
//     they are kept only as a readable statement of the inclusive minimum. The
//     mutant that DOES survive on that line is :1298:15 (`arity.min > 0` ->
//     `>= 0`), and it is EQUIVALENT: parseFuncCall seeds args with `first`, so
//     len(args) >= 1 always and `len(args) < 0` is unreachable. Recorded in
//     tools/mutation-scope.md for adjudication when this package is gated.
func TestParseDepthAndArityBoundsAreInclusive(t *testing.T) {
	// Nesting via parentheses: each pair costs one parseExpr and one
	// parseFactor frame, so build to the limit and one past it.
	nest := func(depth int) string {
		return strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)
	}

	// 99 and 100 are MEASURED, not chosen: each paren level costs one
	// parseExpr frame and one parseFactor frame, so 99 levels reach exactly
	// maxExprDepth(200) and 100 exceed it. The pair has to sit ON that edge —
	// an earlier version used 90 and 500, comfortably either side, and BOTH
	// `>` -> `>=` mutants survived it because the two comparisons only differ
	// at exactly 200.
	//
	// If the frames-per-level ever changes these numbers move with it. That is
	// the point: the observable contract is how deep an expression may be, and
	// a change to it should require someone to look.
	t.Run("nesting exactly at the limit parses", func(t *testing.T) {
		if _, err := rules.Parse(nest(99)); err != nil {
			t.Errorf("Parse(99-deep) = %v, want it accepted — the depth limit is inclusive", err)
		}
	})

	t.Run("nesting one past the limit is rejected", func(t *testing.T) {
		_, err := rules.Parse(nest(100))
		if err == nil {
			t.Fatal("Parse(100-deep) succeeded, want the depth limit to bite one step past")
		}
		if !strings.Contains(err.Error(), "nested too deeply") {
			t.Errorf("Parse(100-deep) = %v, want the DEPTH limit to be what rejects it", err)
		}
	})

	t.Run("a long FLAT expression is not mistaken for a deep one", func(t *testing.T) {
		// 300 terms, nesting depth ~3. This is the case that catches a depth
		// counter which increments but never unwinds: it would accumulate
		// past maxExprDepth on breadth alone and reject a perfectly ordinary
		// expression as "nested too deeply".
		flat := "1" + strings.Repeat("+1", 300)
		if _, err := rules.Parse(flat); err != nil {
			t.Errorf("Parse(301 flat terms) = %v, want it accepted — depth is about NESTING, not length", err)
		}

		// PARENTHESISED siblings, which is the shape that catches a leaked
		// counter. Bare terms do not: parseExpr is entered ONCE for a flat
		// sum and loops over the terms, so a leak in its own unwind never
		// compounds. Each "(1)" is its own parseExpr/parseFactor pair, so 150
		// of them leak 300 and trip a limit of 200 — while nesting stays at 2.
		siblings := "(1)" + strings.Repeat("+(1)", 150)
		if _, err := rules.Parse(siblings); err != nil {
			t.Errorf("Parse(151 parenthesised siblings) = %v, want it accepted — "+
				"151 shallow groups are not 300 levels of nesting", err)
		}
	})

	t.Run("a function called with exactly its minimum arity is accepted", func(t *testing.T) {
		for _, src := range []string{"max(1, 2)", "min(1, 2)"} {
			if _, err := rules.Parse(src); err != nil {
				t.Errorf("Parse(%q) = %v, want it accepted — the minimum is inclusive", src, err)
			}
		}
	})

	t.Run("a function called below its minimum arity is rejected", func(t *testing.T) {
		for _, src := range []string{"max(1)", "min(1)"} {
			if _, err := rules.Parse(src); err == nil {
				t.Errorf("Parse(%q) succeeded, want it rejected — below the minimum arity", src)
			}
		}
	})
}

// TestIdentifierCharsetBoundsAreInclusive pins isIdentStart's four range ends.
//
// `c >= 'A' && c <= 'Z'` and the same for lowercase (expr.go:724) each carry a
// CONDITIONALS_BOUNDARY mutant. THREE survived — :24, :36 and :62. The fourth,
// :50 (`c >= 'a'`), was already dead: every ruleset attribute beginning with
// "a" kills it through isValidIdentName. The three that lived did so because
// every identifier in the suite starts comfortably inside the ranges, where
// `>=` and `>` agree. Loosen any one end by a character and
// exactly one letter of the alphabet stops being a legal identifier start —
// an attribute named "Zeal" or "armor" would fail to parse, and nothing would
// have caught it.
func TestIdentifierCharsetBoundsAreInclusive(t *testing.T) {
	// The four range ends, each exactly on the bound.
	for _, name := range []string{"A", "Z", "a", "z", "_"} {
		t.Run("@"+name, func(t *testing.T) {
			if _, err := rules.Parse("@" + name); err != nil {
				t.Errorf("Parse(%q) = %v, want it accepted — %q is a legal identifier start",
					"@"+name, err, name)
			}
		})
	}

	// One step outside each range, so a bound shifted the permissive way is
	// caught too. '@' is 'A'-1, '[' is 'Z'+1, '`' is 'a'-1, '{' is 'z'+1.
	//
	// Three of these four bite. `@` does NOT: widen the bound to `c >= '@'`
	// and "@@" lexes as ONE identifier, so the parse still fails — at
	// "unexpected identifier", for an unrelated reason. Kept as documentation
	// of the range, not as a pin.
	for _, name := range []string{"@", "[", "`", "{"} {
		t.Run("reject/"+name, func(t *testing.T) {
			if _, err := rules.Parse("@" + name); err == nil {
				t.Errorf("Parse(%q) succeeded, want it rejected — %q is not a letter",
					"@"+name, name)
			}
		})
	}
}
