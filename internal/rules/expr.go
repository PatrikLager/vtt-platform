package rules

import (
	"fmt"
	"strconv"
)

// Expression grammar (v2 — CLOSED grammar, extended from v1 with scoped
// references and expression-sized dice per spec §5; see spec §11 for why
// further extensions beyond this are deliberately deferred):
//
//	expr    := term (('+'|'-') term)*
//	term    := factor (('*'|'/') factor)*
//	factor  := primary ('d' primary)?
//	primary := INT | DICE | ref | func | '(' expr ')'
//	DICE    := INT 'd' INT             (literal fast path: count in [1, 100], sides in [1, 1000], checked at parse time)
//	ref     := ('@'|'#') (scope '.')? IDENT   ('@' = attribute, '#' = resource current)
//	scope   := 'caster' | 'target'
//	func    := ('floor'|'max'|'min'|'half') '(' expr (',' expr)* ')'
//	IDENT   := [A-Za-z_][A-Za-z0-9_]*
//
// factor's optional trailing "'d' primary" is DICE's expression-sized form
// (spec §5): count (the left primary) and sides (the right primary) may be
// ANY primary — a ref, a parenthesized expr, a func call, or another dice —
// not just an integer literal, e.g. '(@caster.dice_count)d(@caster.dice_faces)'
// or '1d(@caster.dice_faces)'. 'd' binds at the factor level — tighter than
// '*'/'/' and '+'/'-' — so '1d6+2' parses as '(1d6)+2', and '2*3d6' parses
// as '2*(3d6)' (see TestEvalDicePrecedence, TestEvalDicePrecedenceDiscriminates).
//
// Dice chaining WITHOUT parentheses is a PARSE ERROR, not a nesting rule
// (P10 task-1 review, controller ruling — an earlier draft of this grammar
// right-recursed the sides operand, silently nesting an unparenthesized
// chain; that is REJECTED now). The reason: a dice operand of a bare 'd'
// may not itself be a bare dice — whether reached via literal fusion
// ('2d3d4': the lexer fuses '2d3' first, so this would silently mean
// '(2d3)d4') or via a further bare 'd' ('2 d 3 d 4' — identical modulo
// whitespace, but this would silently mean '2d(3d4)', since parsePrimary
// has no memory of how many spaces separated the tokens). The SAME
// characters meaning two different things purely because of invisible
// whitespace is exactly the implicit-contract class spec §2 decision 4
// ("no new implicit contracts") bans, so BOTH shapes — and every
// fused/spaced permutation of them — are a ParseError naming the
// ambiguity: "ambiguous dice chain: parenthesize (e.g. (2d3)d4 or
// 2d(3d4))" (see TestParseDiceChainRejectedWithoutParens). Parenthesized
// forms remain fully legal and disambiguate exactly as written — '(2d3)d4'
// and '2d(3d4)' each route their inner dice through parseExpr
// (independently depth-guarded, see maxExprDepth), so wrapping in parens
// is never merely cosmetic; it is what makes a multi-level dice expression
// well-defined at all (see TestEvalDiceChainParenthesizedFormsDiscriminate
// for a roller-call-recording proof that the two parenthesized forms
// really do roll in a different order/shape, not just happen to total the
// same). This rule also closes the CRITICAL stack-overflow hazard the
// earlier right-recursive design had: rejecting an unparenthesized chain
// happens the moment a SECOND 'd' is seen, independent of how long the
// chain is — see TestParseRejectsDeepBareDiceChain — and parseFactor is,
// independently, now depth-accounted exactly like parseExpr as a second,
// redundant bound (defense in depth, not the primary fix).
//
// Bounds (count 1..100, sides 1..1000) apply everywhere, but WHEN they are
// checked depends on how the operand was written: an operand that is a bare
// INT token — whether via the old literal-fusion fast path ('2d6', unchanged
// from v1) or a literal reached through the new factor-'d'-factor form
// ('(1+1)d1001', '(101)d6') — gets its bounds check at PARSE time, exactly
// as v1's pure-literal 'NdM' always did (see
// TestParseExpressionSizedDiceLiteralBoundsStillAtParseTime). An operand
// that is anything else (a ref, a paren-wrapped expression, a func call, a
// nested dice) is not a known value until Eval/EvalScoped runs it, so ITS
// bounds check defers to eval time, with a clean rules error naming the
// COMPUTED value (see TestEvalDiceEvalTimeBounds) — see
// minDiceCount/maxDiceCount/minDiceSides/maxDiceSides. Every integer
// literal is also range-checked against Go's int type at parse time — an
// out-of-range literal is a ParseError naming its position, never silently
// clamped.
//
// Tokenizing 'd' (the ambiguity the task brief calls out directly: is 'd6'
// after a digit run the start of a dice literal, or could "d6" ever be a
// plain identifier?) is resolved in two layers:
//
//  1. lexNumber (the digit-run scanner) fuses an immediately-adjacent
//     'd'+digit-run into a single DICE token ONLY when at least one digit
//     follows 'd' with no intervening whitespace — i.e. only v1's exact
//     literal shape ('2d6', '100d1000'). This is the unchanged v1 fast
//     path: one token, parse-time bounds, no lookahead needed anywhere
//     else. If 'd' is NOT followed by a digit (e.g. '1d(...)', '1d @x',
//     bare '1d' at end of input), lexNumber does not consume 'd' at all —
//     it returns just the digit run as a plain INT, leaving 'd' for the
//     next token.
//  2. Everywhere else, 'd' lexes as an ordinary identifier via the normal
//     identifier scanner (maximal munch: 'd', 'd6', 'd100', and 'dice' all
//     lex as single IDENT tokens, no different from any other identifier).
//     The PARSER decides what an identifier token means based on WHERE it
//     appears: parseFactor, having just parsed a primary, peeks at the
//     next token — if it is an identifier that is either exactly "d" or
//     matches 'd' followed by digits-only ('d6', 'd20', ...), THAT token is
//     the dice operator (with, in the digits-only case, a fused literal
//     sides count split back out of it). This repurposing can never
//     invalidate a previously-valid v1 parse: a bare identifier in this
//     exact position — immediately after a complete factor, not preceded by
//     '@'/'#' — was ALREADY a parse error in v1 (bare identifiers are only
//     legal as function calls, which require '(' next). An identifier like
//     "d6" remains completely ordinary as a REF NAME ('@d6', '#d6')
//     precisely because that code path (parsePrimary's '@'/'#' branches)
//     consumes the identifier token directly as the ref's name and never
//     returns through parseFactor's trailing-'d' check for THAT token — see
//     TestParseBareDIdentifierStillAvailableAsRefName and
//     TestParseBareDAloneAtStartStillErrors.
//
// Scoped refs (spec §5): a ref may be prefixed with an actor scope,
// '@caster.vim' or '#target.vigor', naming which of EvalScoped's two actor
// contexts (caster, target) the identifier resolves against; a ref with no
// scope prefix ('@brawn') is bare, exactly like every v1 ref. Only the
// exact lowercase words 'caster' and 'target' are valid scopes — anything
// else before a '.' ('@self.x', '@foo.x') is a ParseError naming the
// position of the offending word, and a '.' with no identifier after it
// ('@caster.') is likewise a ParseError. An identifier that merely happens
// to be spelled "caster" or "target" with NO following '.' is just a bare
// ref of that name ('@caster' == the attribute literally named "caster"),
// not a scope. Parse ACCEPTS a scoped or bare ref in every position — which
// positions REQUIRE a scope (two-actor: resolution roll/vs, outcome
// effects) and which FORBID one (single-actor: threshold when,
// default_max_expr) is the v2 loader's job (Task 2), using Expr.Scopes() to
// inspect what is present in a given expression. The interpreter enforces
// its own half of this contract defensively, independent of the loader:
// EvalScoped errors on any bare ref (an unscoped identifier has no way to
// pick a context), and Eval errors on any scoped ref (a single-actor
// position has no second context to resolve it against) — the loader
// should already have rejected a wrong-position ref at load time, but
// neither evaluator silently guesses if that check is ever bypassed.
//
// Arity beyond what the grammar production alone expresses (checked at
// parse time, so it is part of the LOAD-time surface, not a runtime
// surprise): max and min take 2 or more arguments; floor and half take
// exactly 1. floor is provided for symmetry with a possible future
// non-integer format — since this grammar is integer-only and '/' already
// floors (see below), floor(x) is the identity function over its single
// argument.
//
// Arithmetic is integer-only. '/' is floor division, not Go's
// truncate-toward-zero division: floor(-7/2) = -4, not -3. half(x) is
// sugar for floor(x/2), so half(-7) = -4 too. Both are pinned by negative-
// operand tests (see TestEvalFloorDivisionNegative and
// TestEvalHalfNegative in expr_test.go) — this is a deliberate, documented
// choice, not an accident of Go's operator semantics.
//
// Dice (NdM, including its expression-sized form) are resolved through the
// caller-supplied Roller at Eval/EvalScoped time; Parse never rolls
// anything. NOTE (v1 load-time restriction, unchanged by v2): although the
// grammar permits DICE anywhere a factor is legal, the loader (load.go)
// REJECTS dice in the two expression positions that are evaluated without
// recording their rolls onto AbilityUsed.rolls — a resource's
// default_max_expr and a threshold's `when` — because a die there would draw
// from the Roller unrecorded, breaking spec §2 decision 3's rolled-once-
// recorded-forever testimony contract. Dice remain legal in attack rolls and
// resource_change delta_expr, which ARE recorded. See Expr.HasDice.
// A bare '@'-ref reads the attrs map passed to Eval (or, in EvalScoped, the
// Attrs of whichever EvalContext its scope selects);
// a bare '#'-ref reads the resources map the same way (a resource's CURRENT
// value only — v1 has no way to reference a resource's max from inside an
// expression). Refs are not resolved against a ruleset's declared names
// until Load (load.go) — Parse is context-free and only checks
// lexical/grammatical validity.

// Scope identifies which actor context a ref resolves against in a
// two-actor evaluation position (EvalScoped): ScopeCaster or ScopeTarget
// for a ref written '@caster.x'/'@target.x' (or with '#'), ScopeNone for a
// bare, unscoped ref ('@x') — legal only where the format allows a bare ref
// to mean "this expression's sole owner" (v1's rule, and v2's single-actor
// positions per spec §5). Recorded on every ref node at parse time;
// Expr.Scopes() exposes it for the v2 loader's (Task 2) position-legality
// checks.
type Scope int

const (
	ScopeNone Scope = iota
	ScopeCaster
	ScopeTarget
)

// scopeName renders s for error messages and ref-display strings ("caster",
// "target", or "" for ScopeNone — callers needing a bare-ref display omit
// the scope entirely rather than printing an empty one; see refDisplay).
func scopeName(s Scope) string {
	switch s {
	case ScopeCaster:
		return "caster"
	case ScopeTarget:
		return "target"
	default:
		return ""
	}
}

// parseScopeName maps a scope word to its Scope, or reports ok=false for
// anything other than the exact lowercase words "caster"/"target" — the
// only two valid scopes (spec §5).
func parseScopeName(s string) (Scope, bool) {
	switch s {
	case "caster":
		return ScopeCaster, true
	case "target":
		return ScopeTarget, true
	default:
		return ScopeNone, false
	}
}

// EvalContext bundles one actor's numeric state for EvalScoped: Attrs
// mirrors Eval's attrs parameter (attribute name -> value), Resources
// mirrors Eval's resources parameter (resource name -> current value). A
// nil map behaves exactly like an empty one — every lookup misses, reported
// as an "unknown attribute/resource" eval error naming the identifier.
type EvalContext struct {
	Attrs     map[string]int
	Resources map[string]int
}

// Roller rolls dice. Production code gets a crypto-seeded implementation
// (Task 5); tests use fixed/deterministic ones. Roll must return exactly n
// results, each in [1, sides], plus their sum as total — callers that need
// to record individual die results (the AbilityUsed contract event carries
// results[] + total, see contract/vtt/v1/events.proto) can do so from the
// returned slice without re-deriving it from total.
type Roller interface {
	Roll(n, sides int) (results []int, total int)
}

// nodeKind tags which Node field group is populated.
type nodeKind int

const (
	nodeInt nodeKind = iota
	nodeDice
	nodeAttrRef
	nodeResourceRef
	nodeBinOp
	nodeFunc
)

// Node is one AST node of a parsed expression. Only the fields relevant to
// Kind are populated; the rest are zero. Unexported — callers interact with
// expressions through Expr, Parse, Eval, and EvalScoped only.
type Node struct {
	kind nodeKind

	// nodeInt
	intValue int

	// nodeDice — count and sides are themselves sub-expressions (spec §5,
	// expression-sized dice). v1's pure-literal 'NdM' (and any other purely
	// literal dice, however it was written) is represented uniformly as two
	// nodeInt leaves, so eval has exactly one dice code path regardless of
	// how the dice was written or how deeply its operands nest.
	diceCount, diceSides *Node

	// nodeAttrRef, nodeResourceRef
	ident string
	scope Scope // ScopeNone for a bare (unscoped) ref — see Scope's doc.

	// nodeBinOp
	op          byte // '+', '-', '*', '/'
	left, right *Node

	// nodeFunc
	funcName string
	args     []*Node
}

// Expr is a parsed expression: the AST root plus the original source text
// (kept for error messages and for reproducing the expression in
// diagnostics — never re-parsed).
type Expr struct {
	root *Node
	src  string
}

// String returns the original source text Parse was given.
func (e *Expr) String() string {
	if e == nil {
		return ""
	}
	return e.src
}

// Refs returns the distinct attribute ('@') and resource ('#') identifiers
// referenced anywhere in the expression — including inside a dice node's
// count/sides sub-expressions (spec §5, expression-sized dice; v1's
// always-literal dice never had anything to walk into here) — in
// first-occurrence order. Used by the loader to cross-check every
// expression's identifiers against the ruleset manifest's declared
// attribute/resource names (spec §5) — Parse itself does not know about
// declared names. Scope (see Scopes) is not reflected here: a scoped and a
// bare ref to the same name collapse to one entry, exactly like two bare
// refs to the same name always have.
func (e *Expr) Refs() (attrs []string, resources []string) {
	if e == nil || e.root == nil {
		return nil, nil
	}
	seenAttr := map[string]bool{}
	seenRes := map[string]bool{}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.kind {
		case nodeAttrRef:
			if !seenAttr[n.ident] {
				seenAttr[n.ident] = true
				attrs = append(attrs, n.ident)
			}
		case nodeResourceRef:
			if !seenRes[n.ident] {
				seenRes[n.ident] = true
				resources = append(resources, n.ident)
			}
		case nodeBinOp:
			walk(n.left)
			walk(n.right)
		case nodeFunc:
			for _, a := range n.args {
				walk(a)
			}
		case nodeDice:
			walk(n.diceCount)
			walk(n.diceSides)
		}
	}
	walk(e.root)
	return attrs, resources
}

// Scopes returns the Scope of every ref (attribute or resource) node
// encountered during a walk of the expression — including refs nested
// inside dice count/sides sub-expressions — in the same traversal order
// Refs uses, one entry per ref OCCURRENCE (not deduplicated by name the way
// Refs is: a name referenced twice with different scopes, or the same
// scope, produces two entries). ScopeNone entries mark bare refs. This is
// the v2 loader's (Task 2) primitive for load-time position-legality
// validation — a two-actor position rejects any ScopeNone entry, a
// single-actor position rejects any non-ScopeNone entry (spec §5).
func (e *Expr) Scopes() []Scope {
	if e == nil || e.root == nil {
		return nil
	}
	var out []Scope
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.kind {
		case nodeAttrRef, nodeResourceRef:
			out = append(out, n.scope)
		case nodeBinOp:
			walk(n.left)
			walk(n.right)
		case nodeFunc:
			for _, a := range n.args {
				walk(a)
			}
		case nodeDice:
			walk(n.diceCount)
			walk(n.diceSides)
		}
	}
	walk(e.root)
	return out
}

// ScopedRef is one ref occurrence exactly as ScopedRefs walks it: which
// sigil ('@' or '#'), which Scope (ScopeNone for bare), and which name.
type ScopedRef struct {
	Sigil byte
	Scope Scope
	Name  string
}

// ScopedRefs returns every ref (attribute or resource) OCCURRENCE in the
// expression — including refs nested inside dice count/sides
// sub-expressions — in AST-walk order, each carrying its sigil, Scope, and
// name TOGETHER (P10 task-1 review, item 4). Refs (deduped by name, split
// into attrs/resources) and Scopes (one Scope per occurrence, same order,
// but scope-only) cannot be zipped back into (name, scope) pairs — Refs
// dedupes and Scopes doesn't carry names, so their lengths and orders
// diverge as soon as an expression has more than one ref. ScopedRefs is
// the single primitive that keeps sigil+scope+name atomic per occurrence;
// it is what the v2 loader (Task 2) should use for position-legality
// validation instead of trying to recombine Refs/Scopes.
func (e *Expr) ScopedRefs() []ScopedRef {
	if e == nil || e.root == nil {
		return nil
	}
	var out []ScopedRef
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.kind {
		case nodeAttrRef:
			out = append(out, ScopedRef{Sigil: '@', Scope: n.scope, Name: n.ident})
		case nodeResourceRef:
			out = append(out, ScopedRef{Sigil: '#', Scope: n.scope, Name: n.ident})
		case nodeBinOp:
			walk(n.left)
			walk(n.right)
		case nodeFunc:
			for _, a := range n.args {
				walk(a)
			}
		case nodeDice:
			walk(n.diceCount)
			walk(n.diceSides)
		}
	}
	walk(e.root)
	return out
}

// HasDice reports whether the expression contains any DICE node. The loader
// uses it to reject dice in the two expression positions that are evaluated
// WITHOUT recording their rolls onto AbilityUsed.rolls — threshold `when` and
// resource `default_max_expr` — a v1 restriction that preserves spec §2
// decision 3's "rolled once, recorded forever" testimony contract (dice
// elsewhere, in attack rolls and resource_change delta_expr, ARE recorded).
func (e *Expr) HasDice() bool {
	if e == nil || e.root == nil {
		return false
	}
	var walk func(n *Node) bool
	walk = func(n *Node) bool {
		if n == nil {
			return false
		}
		switch n.kind {
		case nodeDice:
			return true
		case nodeBinOp:
			return walk(n.left) || walk(n.right)
		case nodeFunc:
			for _, a := range n.args {
				if walk(a) {
					return true
				}
			}
		}
		return false
	}
	return walk(e.root)
}

// ParseError is returned by Parse on any grammar or lexical violation. Pos
// is the byte offset into the source where the problem was found (or, for
// "expected X but ran out of input" errors, the offset of end-of-input).
type ParseError struct {
	Pos int
	Msg string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("rules: expr: parse error at position %d: %s", e.Pos, e.Msg)
}

// Parse parses s per the grammar documented above. Parse errors name the
// byte position and the offending token/text (via *ParseError).
func Parse(s string) (*Expr, error) {
	p := &parser{lex: &lexer{src: s}}
	if err := p.advance(); err != nil {
		return nil, err
	}
	root, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokEOF {
		return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("unexpected trailing input %s", describeTok(p.tok))}
	}
	return &Expr{root: root, src: s}, nil
}

// Eval evaluates e against attrs (attribute name -> value), resources
// (resource name -> current value), and roller (used for DICE nodes) — a
// SINGLE-actor position (spec §5: threshold when, default_max_expr, and any
// other position the format declares single-actor). Integer division
// floors (see grammar doc above); division by zero and unresolved
// identifiers are runtime errors, not panics. Every ref in e must be bare
// (ScopeNone) — a scoped ref ('@caster.x') is a defense-in-depth eval error
// naming the ref, since a single-actor position has no second context to
// resolve it against (the v2 loader should already reject a scoped ref
// here at load time; see Expr.Scopes).
func Eval(e *Expr, attrs map[string]int, resources map[string]int, roller Roller) (int, error) {
	if e == nil || e.root == nil {
		return 0, fmt.Errorf("rules: expr: eval on nil expression")
	}
	resolve := func(n *Node) (int, error) {
		if n.scope != ScopeNone {
			return 0, fmt.Errorf("rules: expr: scoped reference %s not allowed here: Eval is a single-actor position, write a bare reference instead", refDisplay(n))
		}
		switch n.kind {
		case nodeAttrRef:
			v, ok := attrs[n.ident]
			if !ok {
				return 0, fmt.Errorf("rules: expr: unknown attribute %q", n.ident)
			}
			return v, nil
		case nodeResourceRef:
			v, ok := resources[n.ident]
			if !ok {
				return 0, fmt.Errorf("rules: expr: unknown resource %q", n.ident)
			}
			return v, nil
		default:
			return 0, fmt.Errorf("rules: expr: internal error: resolver called on non-ref node kind %d", n.kind)
		}
	}
	return evalTree(e.root, resolve, roller)
}

// EvalScoped evaluates e against caster and target actor contexts — a
// TWO-actor position (spec §5: resolution roll/vs, outcome effect
// expressions). Every ref in e must carry a scope (ScopeCaster or
// ScopeTarget) naming which context it resolves against; a bare ref
// ('@x') is a defense-in-depth eval error naming the ref (the v2 loader
// should already require a scope here at load time; see Expr.Scopes). An
// expression with no refs at all (pure arithmetic/dice/func) needs neither
// context and evaluates the same as it would under Eval.
func EvalScoped(e *Expr, caster, target EvalContext, roller Roller) (int, error) {
	if e == nil || e.root == nil {
		return 0, fmt.Errorf("rules: expr: eval on nil expression")
	}
	resolve := func(n *Node) (int, error) {
		var ctx EvalContext
		switch n.scope {
		case ScopeCaster:
			ctx = caster
		case ScopeTarget:
			ctx = target
		default:
			return 0, fmt.Errorf("rules: expr: bare reference %s not allowed here: EvalScoped is a two-actor position, write @caster.%s or @target.%s", refDisplay(n), n.ident, n.ident)
		}
		switch n.kind {
		case nodeAttrRef:
			v, ok := ctx.Attrs[n.ident]
			if !ok {
				return 0, fmt.Errorf("rules: expr: unknown attribute %q for %s", n.ident, scopeName(n.scope))
			}
			return v, nil
		case nodeResourceRef:
			v, ok := ctx.Resources[n.ident]
			if !ok {
				return 0, fmt.Errorf("rules: expr: unknown resource %q for %s", n.ident, scopeName(n.scope))
			}
			return v, nil
		default:
			return 0, fmt.Errorf("rules: expr: internal error: resolver called on non-ref node kind %d", n.kind)
		}
	}
	return evalTree(e.root, resolve, roller)
}

// refDisplay renders n (a nodeAttrRef or nodeResourceRef) back into
// expression syntax for error messages: "@brawn", "#pool_a",
// "@caster.vim", "#target.vigor".
func refDisplay(n *Node) string {
	sigil := "@"
	if n.kind == nodeResourceRef {
		sigil = "#"
	}
	if n.scope == ScopeNone {
		return sigil + n.ident
	}
	return sigil + scopeName(n.scope) + "." + n.ident
}

// refResolver resolves one ref node (nodeAttrRef or nodeResourceRef) to its
// numeric value, or returns an error. Eval and EvalScoped each supply a
// resolver closure over their own context(s); evalTree's walk — arithmetic,
// functions, and dice (including a dice's count/sides sub-expressions,
// which may themselves contain refs) — is otherwise IDENTICAL between the
// two entry points, so there is exactly one place that implements "how an
// expression evaluates" regardless of which position invoked it.
type refResolver func(n *Node) (int, error)

func evalTree(n *Node, resolve refResolver, roller Roller) (int, error) {
	switch n.kind {
	case nodeInt:
		return n.intValue, nil

	case nodeAttrRef, nodeResourceRef:
		return resolve(n)

	case nodeDice:
		count, err := evalTree(n.diceCount, resolve, roller)
		if err != nil {
			return 0, err
		}
		sides, err := evalTree(n.diceSides, resolve, roller)
		if err != nil {
			return 0, err
		}
		if count < minDiceCount || count > maxDiceCount {
			return 0, fmt.Errorf("rules: expr: dice count %d out of bounds [%d, %d]", count, minDiceCount, maxDiceCount)
		}
		if sides < minDiceSides || sides > maxDiceSides {
			return 0, fmt.Errorf("rules: expr: dice sides %d out of bounds [%d, %d]", sides, minDiceSides, maxDiceSides)
		}
		if roller == nil {
			return 0, fmt.Errorf("rules: expr: dice %dd%d requires a non-nil Roller", count, sides)
		}
		_, total := roller.Roll(count, sides)
		return total, nil

	case nodeBinOp:
		l, err := evalTree(n.left, resolve, roller)
		if err != nil {
			return 0, err
		}
		r, err := evalTree(n.right, resolve, roller)
		if err != nil {
			return 0, err
		}
		switch n.op {
		case '+':
			return l + r, nil
		case '-':
			return l - r, nil
		case '*':
			return l * r, nil
		case '/':
			if r == 0 {
				return 0, fmt.Errorf("rules: expr: division by zero")
			}
			return floorDiv(l, r), nil
		default:
			return 0, fmt.Errorf("rules: expr: internal error: unknown operator %q", n.op)
		}

	case nodeFunc:
		vals := make([]int, len(n.args))
		for i, a := range n.args {
			v, err := evalTree(a, resolve, roller)
			if err != nil {
				return 0, err
			}
			vals[i] = v
		}
		switch n.funcName {
		case "floor":
			return vals[0], nil
		case "half":
			return floorDiv(vals[0], 2), nil
		case "max":
			m := vals[0]
			for _, v := range vals[1:] {
				if v > m {
					m = v
				}
			}
			return m, nil
		case "min":
			m := vals[0]
			for _, v := range vals[1:] {
				if v < m {
					m = v
				}
			}
			return m, nil
		default:
			return 0, fmt.Errorf("rules: expr: internal error: unknown function %q", n.funcName)
		}

	default:
		return 0, fmt.Errorf("rules: expr: internal error: unknown node kind %d", n.kind)
	}
}

// floorDiv computes floor(a/b) for b != 0 — Go's native '/' truncates
// toward zero (-7/2 == -3); this adjusts by 1 whenever truncation and
// flooring disagree, i.e. whenever there's a nonzero remainder AND the
// operands have different signs.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// --- lexer ---

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokInt
	tokDice
	tokIdent
	tokAt
	tokHash
	tokDot
	tokLParen
	tokRParen
	tokComma
	tokPlus
	tokMinus
	tokStar
	tokSlash
)

type token struct {
	kind             tokenKind
	pos              int
	text             string
	intVal           int
	diceN, diceSides int
}

type lexer struct {
	src string
	pos int
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isValidIdentName reports whether s matches the expression grammar's
// IDENT production ([A-Za-z_][A-Za-z0-9_]*) — reused by load.go to reject
// manifest attribute/defense/resource names that could never actually be
// referenced via '@'/'#' in an expression (e.g. a hyphenated name would
// lex as subtraction, not one identifier — see expr.go's grammar doc and
// the resource-naming note in the task-4 report). Built on the lexer's own
// isIdentStart/isIdentCont so this check can never drift from what Parse
// actually accepts after a sigil — a second, hand-written regex duplicating
// the same charset would be exactly the kind of parallel-truth this
// package's schema/loader cross-checks exist to prevent.
func isValidIdentName(s string) bool {
	if s == "" || !isIdentStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isIdentCont(s[i]) {
			return false
		}
	}
	return true
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		switch l.src[l.pos] {
		case ' ', '\t', '\n', '\r':
			l.pos++
		default:
			return
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skipSpace()
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, pos: l.pos}, nil
	}
	start := l.pos
	c := l.src[l.pos]
	switch {
	case isDigit(c):
		return l.lexNumber()
	case isIdentStart(c):
		return l.lexIdent()
	case c == '@':
		l.pos++
		return token{kind: tokAt, pos: start, text: "@"}, nil
	case c == '#':
		l.pos++
		return token{kind: tokHash, pos: start, text: "#"}, nil
	case c == '.':
		l.pos++
		return token{kind: tokDot, pos: start, text: "."}, nil
	case c == '(':
		l.pos++
		return token{kind: tokLParen, pos: start, text: "("}, nil
	case c == ')':
		l.pos++
		return token{kind: tokRParen, pos: start, text: ")"}, nil
	case c == ',':
		l.pos++
		return token{kind: tokComma, pos: start, text: ","}, nil
	case c == '+':
		l.pos++
		return token{kind: tokPlus, pos: start, text: "+"}, nil
	case c == '-':
		l.pos++
		return token{kind: tokMinus, pos: start, text: "-"}, nil
	case c == '*':
		l.pos++
		return token{kind: tokStar, pos: start, text: "*"}, nil
	case c == '/':
		l.pos++
		return token{kind: tokSlash, pos: start, text: "/"}, nil
	default:
		return token{}, &ParseError{Pos: start, Msg: fmt.Sprintf("unexpected character %q", string(c))}
	}
}

func (l *lexer) lexIdent() (token, error) {
	start := l.pos
	for l.pos < len(l.src) && isIdentCont(l.src[l.pos]) {
		l.pos++
	}
	return token{kind: tokIdent, pos: start, text: l.src[start:l.pos]}, nil
}

// lexNumber scans a leading integer and, if immediately followed by 'd'
// AND at least one digit (checked via one-byte lookahead BEFORE committing
// to consume 'd' — see expr.go's "Tokenizing 'd'" doc above), folds the
// whole thing into a single DICE token (INT 'd' INT, the v1 literal fast
// path — unchanged bounds/behavior). If 'd' is present but NOT followed by
// a digit (e.g. '1d(...)', '1d @x', a bare '1d' at end of input), lexNumber
// does NOT consume 'd' at all: it returns just the digit run as a plain
// INT, leaving 'd' for the next token — where the parser's factor-level
// dice-operator check (expr.go's "Tokenizing 'd'" doc, point 2) takes over,
// because that 'd' is now the start of the NEW expression-sized dice form
// (factor 'd' factor), not v1's pure-literal shape.
//
// Every integer literal parsed here (a bare INT, or either half of a
// literal DICE) goes through strconv.Atoi and its error is checked — an
// out-of-range literal (more digits than fit in an int) is a ParseError
// naming the literal's position, never a silently-clamped value.
func (l *lexer) lexNumber() (token, error) {
	start := l.pos
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.pos++
	}
	countText := l.src[start:l.pos]
	count, err := strconv.Atoi(countText)
	if err != nil {
		return token{}, &ParseError{Pos: start, Msg: fmt.Sprintf("integer literal %q is out of range", countText)}
	}

	if l.pos < len(l.src) && l.src[l.pos] == 'd' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1]) {
		l.pos++ // consume 'd' — a digit is confirmed to follow, so this dice literal is complete
		sidesStart := l.pos
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
		sidesText := l.src[sidesStart:l.pos]
		sides, err := strconv.Atoi(sidesText)
		if err != nil {
			return token{}, &ParseError{Pos: sidesStart, Msg: fmt.Sprintf("integer literal %q is out of range", sidesText)}
		}
		if count < minDiceCount || count > maxDiceCount {
			return token{}, &ParseError{Pos: start, Msg: fmt.Sprintf("dice count %d out of bounds [%d, %d]", count, minDiceCount, maxDiceCount)}
		}
		if sides < minDiceSides || sides > maxDiceSides {
			return token{}, &ParseError{Pos: sidesStart, Msg: fmt.Sprintf("dice sides %d out of bounds [%d, %d]", sides, minDiceSides, maxDiceSides)}
		}
		return token{kind: tokDice, pos: start, text: l.src[start:l.pos], diceN: count, diceSides: sides}, nil
	}
	return token{kind: tokInt, pos: start, text: countText, intVal: count}, nil
}

// Dice bounds (enforced at parse time, not just documented): generous
// beyond any real tabletop need, but bounded so a caller's Roller (Task 5)
// is never asked to produce an unbounded number of results from something
// like "1000000000d1000000000" — that literal parses cleanly today
// (well within int64 range, so strconv.Atoi alone would not catch it) and
// would otherwise demand a billion-element roll.
const (
	minDiceCount = 1
	maxDiceCount = 100
	minDiceSides = 1
	maxDiceSides = 1000
)

// --- parser (recursive descent, one token of lookahead) ---

// maxExprDepth bounds recursion through parseExpr — reached only via
// nested '(' groups or nested function-call arguments. No real ruleset
// expression comes remotely close to this; it exists purely so pathological
// input (e.g. thousands of consecutive '(') fails with a clean ParseError
// instead of exhausting the goroutine stack. A Go stack overflow is a fatal,
// unrecoverable runtime error, not a catchable panic — undiscovered by
// go vet or the race detector, but exactly what a parser fuzz target must
// not allow (task brief: "parser must never panic, only error").
const maxExprDepth = 200

type parser struct {
	lex   *lexer
	tok   token
	depth int
}

func (p *parser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = t
	return nil
}

func describeTok(t token) string {
	if t.kind == tokEOF {
		return "end of input"
	}
	return fmt.Sprintf("%q", t.text)
}

// parseExpr := term (('+'|'-') term)*
func (p *parser) parseExpr() (*Node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expression nested too deeply (max %d)", maxExprDepth)}
	}

	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokPlus || p.tok.kind == tokMinus {
		op := byte('+')
		if p.tok.kind == tokMinus {
			op = '-'
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = &Node{kind: nodeBinOp, op: op, left: left, right: right}
	}
	return left, nil
}

// parseTerm := factor (('*'|'/') factor)*
func (p *parser) parseTerm() (*Node, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokStar || p.tok.kind == tokSlash {
		op := byte('*')
		if p.tok.kind == tokSlash {
			op = '/'
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = &Node{kind: nodeBinOp, op: op, left: left, right: right}
	}
	return left, nil
}

var funcArity = map[string]struct {
	exact int // exact >= 1 means "exactly this many"; 0 means "use min"
	min   int
}{
	"floor": {exact: 1},
	"half":  {exact: 1},
	"max":   {min: 2},
	"min":   {min: 2},
}

// parseFactor := primary ('d' primary)?
//
// The optional trailing "'d' primary" is DICE's expression-sized form
// (spec §5) — see expr.go's grammar doc for the full "Tokenizing 'd'"
// account of how this is distinguished from an ordinary identifier, and
// the "Dice chaining" section for the controller ruling this function
// enforces (P10 task-1 review, item 3): an unparenthesized dice chain is a
// ParseError, not a nesting rule. In short: parsePrimary parses the left
// operand; if the token immediately following it is an identifier shaped
// like "d" or "d"+digits-only, that identifier IS the dice operator (never
// a legal token in this position otherwise — see
// TestParseBareDAloneAtStartStillErrors), and the sides operand is either
// the fused literal digits (the "d"+digits case) or a SINGLE further
// primary (the bare "d" case — deliberately parsePrimary, not a recursive
// parseFactor call, so a bare 'd' can never itself absorb a further 'd').
// Depth-accounted like parseExpr (defense in depth — see the grammar doc's
// "Dice chaining" section for why this is no longer the primary guard
// against unbounded recursion, ruling #3 is).
func (p *parser) parseFactor() (*Node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expression nested too deeply (max %d)", maxExprDepth)}
	}

	leftPos := p.tok.pos
	left, leftViaParen, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	if p.tok.kind != tokIdent {
		return left, nil
	}
	digits, isDiceOp := diceOpSuffix(p.tok.text)
	if !isDiceOp {
		return left, nil
	}
	opPos := p.tok.pos

	// Controller ruling (item 3): left is about to become the dice COUNT
	// operand of a NEW dice. If it is ALREADY a dice — reached via literal
	// fusion, e.g. the "2d3" in "2d3d4" — and did NOT come through
	// explicit parens, that is exactly the ambiguous, whitespace-sensitive
	// shape this rule bans.
	if left.kind == nodeDice && !leftViaParen {
		return nil, ambiguousDiceChainError(leftPos)
	}

	// left is now confirmed to be a legal dice COUNT operand: a literal
	// bound check applies here (not in parsePrimary) because a bare
	// integer is only subject to the dice-count range when it is
	// ACTUALLY used as a count — the same "500" is a perfectly ordinary
	// integer everywhere else in the grammar.
	if left.kind == nodeInt {
		if left.intValue < minDiceCount || left.intValue > maxDiceCount {
			return nil, &ParseError{Pos: leftPos, Msg: fmt.Sprintf("dice count %d out of bounds [%d, %d]", left.intValue, minDiceCount, maxDiceCount)}
		}
	}

	var sides *Node
	if digits == "" {
		// Bare "d": consume it, then parse the sides operand as a SINGLE
		// primary — never a recursive parseFactor call (that recursion,
		// unguarded, was the CRITICAL stack-overflow hazard; see the
		// grammar doc). This is what permits '1d(@caster.dice_faces)'
		// (the parens route through parsePrimary's tokLParen branch,
		// which is fine — parseExpr guards that recursion independently)
		// while refusing to silently chain a further bare 'd'.
		if err := p.advance(); err != nil {
			return nil, err
		}
		sidesPos := p.tok.pos
		var sidesViaParen bool
		sides, sidesViaParen, err = p.parsePrimary()
		if err != nil {
			return nil, err
		}
		// Symmetric to the count check above: sides may not itself be an
		// unparenthesized dice either (e.g. the "2d6" in "1 d 2d6").
		if sides.kind == nodeDice && !sidesViaParen {
			return nil, ambiguousDiceChainError(sidesPos)
		}
		if sides.kind == nodeInt {
			if sides.intValue < minDiceSides || sides.intValue > maxDiceSides {
				return nil, &ParseError{Pos: sidesPos, Msg: fmt.Sprintf("dice sides %d out of bounds [%d, %d]", sides.intValue, minDiceSides, maxDiceSides)}
			}
		}
	} else {
		// "d"+digits (e.g. "d6"): the identifier scanner already consumed
		// the whole thing as one token (see lexIdent) — split the digits
		// back out as a literal sides operand, checked immediately since
		// it is unambiguously a literal here. Never a dice itself, so no
		// chain check needed on this branch's sides.
		sidesPos := opPos + 1
		n, convErr := strconv.Atoi(digits)
		if convErr != nil {
			return nil, &ParseError{Pos: sidesPos, Msg: fmt.Sprintf("integer literal %q is out of range", digits)}
		}
		if n < minDiceSides || n > maxDiceSides {
			return nil, &ParseError{Pos: sidesPos, Msg: fmt.Sprintf("dice sides %d out of bounds [%d, %d]", n, minDiceSides, maxDiceSides)}
		}
		if err := p.advance(); err != nil { // consume the fused "d123" token
			return nil, err
		}
		sides = &Node{kind: nodeInt, intValue: n}
	}

	dice := &Node{kind: nodeDice, diceCount: left, diceSides: sides}

	// Post-construction chain check: if what immediately follows this
	// freshly-built, unparenthesized dice is ALSO a dice operator, the
	// user has written a further ambiguous chain link (e.g. the leftover
	// trailing 'd 4' after building '2d3' out of '2 d 3 d 4') — reject
	// with the same message instead of letting it degrade into a generic
	// "unexpected trailing input" error.
	if p.tok.kind == tokIdent {
		if _, chained := diceOpSuffix(p.tok.text); chained {
			return nil, ambiguousDiceChainError(p.tok.pos)
		}
	}

	return dice, nil
}

// ambiguousDiceChainError is the controller-ruling error (P10 task-1
// review, item 3) for any unparenthesized dice chain. See parseFactor's
// doc and expr.go's grammar doc ("Dice chaining") for the full rationale.
func ambiguousDiceChainError(pos int) error {
	return &ParseError{Pos: pos, Msg: "ambiguous dice chain: parenthesize (e.g. (2d3)d4 or 2d(3d4))"}
}

// diceOpSuffix reports whether an identifier token's text is the dice
// operator: either exactly "d" (sides parsed separately, as a full factor —
// see parseFactor) or "d" followed by one or more digits and NOTHING else
// (a literal sides count fused into the identifier by lexIdent's maximal
// munch — e.g. "d6", "d20"; see expr.go's "Tokenizing 'd'" doc). Returns
// the digit substring (empty for the bare "d" case) and ok=true only for
// these two shapes; any other identifier (including one merely starting
// with 'd', like "dice" or "d6x") is not a dice operator at all.
func diceOpSuffix(text string) (digits string, ok bool) {
	if text == "d" {
		return "", true
	}
	if len(text) > 1 && text[0] == 'd' {
		for i := 1; i < len(text); i++ {
			if !isDigit(text[i]) {
				return "", false
			}
		}
		return text[1:], true
	}
	return "", false
}

// parsePrimary := INT | DICE | ref | func | '(' expr ')'
// ref  := ('@'|'#') (scope '.')? IDENT
//
// This is v1's parseFactor body, renamed: it parses everything BELOW the
// new dice-operator suffix (see parseFactor above), i.e. one atomic value
// before any trailing 'd' is considered.
//
// Returns (node, viaParen, error): viaParen is true ONLY for the
// '(' expr ')' branch. parseFactor uses it to distinguish "this value
// happens to be a dice because of literal fusion or a preceding bare-'d'
// application" (viaParen=false — illegal as a further, unparenthesized
// dice operand, per the controller ruling in parseFactor's doc) from "this
// value is a dice because the caller explicitly wrote parens around it"
// (viaParen=true — always legal; parens are what resolve the ambiguity).
//
// NOTE: there is deliberately no unary '-' (or '+') case here — the
// grammar is closed and has no unary-operator production (see expr.go's
// top doc comment and TestParseRejectsUnaryMinus).
func (p *parser) parsePrimary() (*Node, bool, error) {
	switch p.tok.kind {
	case tokInt:
		n := &Node{kind: nodeInt, intValue: p.tok.intVal}
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		return n, false, nil

	case tokDice:
		n := &Node{
			kind:      nodeDice,
			diceCount: &Node{kind: nodeInt, intValue: p.tok.diceN},
			diceSides: &Node{kind: nodeInt, intValue: p.tok.diceSides},
		}
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		return n, false, nil

	case tokAt:
		n, err := p.parseRef(tokAt)
		return n, false, err

	case tokHash:
		n, err := p.parseRef(tokHash)
		return n, false, err

	case tokIdent:
		n, err := p.parseFuncCall()
		return n, false, err

	case tokLParen:
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, false, err
		}
		if p.tok.kind != tokRParen {
			return nil, false, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expected ')', found %s", describeTok(p.tok))}
		}
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		return inner, true, nil

	default:
		return nil, false, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expected expression, found %s", describeTok(p.tok))}
	}
}

// parseRef parses ('@'|'#') (scope '.')? IDENT — sigilKind is tokAt or
// tokHash, selecting nodeAttrRef vs nodeResourceRef. The first identifier
// after the sigil is tentatively the ref's name; if a '.' follows, that
// first identifier must instead be a valid scope word ("caster"/"target" —
// anything else is a ParseError naming ITS position, per the task brief),
// and the real ref name is the identifier after the '.'. No '.' at all
// means a bare, unscoped ref — exactly v1's shape.
func (p *parser) parseRef(sigilKind tokenKind) (*Node, error) {
	sigilPos := p.tok.pos
	sigilName := "@"
	nodeKindForSigil := nodeAttrRef
	if sigilKind == tokHash {
		sigilName = "#"
		nodeKindForSigil = nodeResourceRef
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, &ParseError{Pos: sigilPos, Msg: fmt.Sprintf("expected identifier after '%s'", sigilName)}
	}
	name1 := p.tok.text
	name1Pos := p.tok.pos
	if err := p.advance(); err != nil {
		return nil, err
	}

	if p.tok.kind != tokDot {
		return &Node{kind: nodeKindForSigil, ident: name1, scope: ScopeNone}, nil
	}

	scope, ok := parseScopeName(name1)
	if !ok {
		return nil, &ParseError{Pos: name1Pos, Msg: fmt.Sprintf("unknown scope %q (valid scopes: caster, target)", name1)}
	}
	dotPos := p.tok.pos
	if err := p.advance(); err != nil { // consume '.'
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, &ParseError{Pos: dotPos, Msg: "expected identifier after '.'"}
	}
	name2 := p.tok.text
	if err := p.advance(); err != nil {
		return nil, err
	}
	return &Node{kind: nodeKindForSigil, ident: name2, scope: scope}, nil
}

func (p *parser) parseFuncCall() (*Node, error) {
	name := p.tok.text
	namePos := p.tok.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind != tokLParen {
		return nil, &ParseError{Pos: namePos, Msg: fmt.Sprintf("unexpected identifier %q (bare identifiers are only valid as function calls: floor/max/min/half)", name)}
	}
	arity, known := funcArity[name]
	if !known {
		return nil, &ParseError{Pos: namePos, Msg: fmt.Sprintf("unknown function %q", name)}
	}
	if err := p.advance(); err != nil { // consume '('
		return nil, err
	}

	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	args := []*Node{first}
	for p.tok.kind == tokComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	if p.tok.kind != tokRParen {
		return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expected ')' or ',', found %s", describeTok(p.tok))}
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	if arity.exact > 0 && len(args) != arity.exact {
		return nil, &ParseError{Pos: namePos, Msg: fmt.Sprintf("%s expects exactly %d argument(s), got %d", name, arity.exact, len(args))}
	}
	if arity.min > 0 && len(args) < arity.min {
		return nil, &ParseError{Pos: namePos, Msg: fmt.Sprintf("%s expects at least %d arguments, got %d", name, arity.min, len(args))}
	}

	return &Node{kind: nodeFunc, funcName: name, args: args}, nil
}
