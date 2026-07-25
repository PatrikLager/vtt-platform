package rules

import (
	"fmt"
	"strconv"
)

// Expression grammar (CLOSED — this is the complete, final v1 grammar; see
// spec §11 for why extensions are deliberately deferred to a v2 format):
//
//	expr    := term (('+'|'-') term)*
//	term    := factor (('*'|'/') factor)*
//	factor  := INT | DICE | ref | func | '(' expr ')'
//	DICE    := INT 'd' INT             (count in [1, 100], sides in [1, 1000])
//	ref     := '@'IDENT (attribute) | '#'IDENT (resource current)
//	func    := ('floor'|'max'|'min'|'half') '(' expr (',' expr)* ')'
//	IDENT   := [A-Za-z_][A-Za-z0-9_]*
//
// DICE bounds (count 1..100, sides 1..1000) are enforced at parse time —
// generous beyond any real tabletop need, but a hard ceiling so a caller's
// Roller (Task 5) is never asked to produce an unbounded roll; see
// minDiceCount/maxDiceCount/minDiceSides/maxDiceSides in lexNumber. Every
// integer literal (a bare INT, or either half of a DICE) is also range-
// checked against Go's int type at parse time — an out-of-range literal is
// a ParseError naming its position, never silently clamped.
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
// Dice (NdM) are resolved through the caller-supplied Roller at Eval time;
// Parse never rolls anything. NOTE (v1 load-time restriction): although the
// grammar permits DICE anywhere a factor is legal, the loader (load.go)
// REJECTS dice in the two expression positions that are evaluated without
// recording their rolls onto AbilityUsed.rolls — a resource's
// default_max_expr and a threshold's `when` — because a die there would draw
// from the Roller unrecorded, breaking spec §2 decision 3's rolled-once-
// recorded-forever testimony contract. Dice remain legal in attack rolls and
// resource_change delta_expr, which ARE recorded. See Expr.HasDice.
// '@'-refs read the attrs map passed to Eval;
// '#'-refs read the resources map (a resource's CURRENT value only — v1 has
// no way to reference a resource's max from inside an expression). Refs
// are not resolved against a ruleset's declared names until Load
// (load.go) — Parse is context-free and only checks lexical/grammatical
// validity.

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
// expressions through Expr, Parse, and Eval only.
type Node struct {
	kind nodeKind

	// nodeInt
	intValue int

	// nodeDice
	diceN, diceSides int

	// nodeAttrRef, nodeResourceRef
	ident string

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
// referenced anywhere in the expression, in first-occurrence order. Used by
// the loader to cross-check every expression's identifiers against the
// ruleset manifest's declared attribute/resource names (spec §5) — Parse
// itself does not know about declared names.
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
		}
	}
	walk(e.root)
	return attrs, resources
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
// (resource name -> current value), and roller (used for DICE nodes).
// Integer division floors (see grammar doc above); division by zero and
// unresolved identifiers are runtime errors, not panics.
func Eval(e *Expr, attrs map[string]int, resources map[string]int, roller Roller) (int, error) {
	if e == nil || e.root == nil {
		return 0, fmt.Errorf("rules: expr: eval on nil expression")
	}
	return evalNode(e.root, attrs, resources, roller)
}

func evalNode(n *Node, attrs, resources map[string]int, roller Roller) (int, error) {
	switch n.kind {
	case nodeInt:
		return n.intValue, nil

	case nodeDice:
		if roller == nil {
			return 0, fmt.Errorf("rules: expr: dice %dd%d requires a non-nil Roller", n.diceN, n.diceSides)
		}
		_, total := roller.Roll(n.diceN, n.diceSides)
		return total, nil

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

	case nodeBinOp:
		l, err := evalNode(n.left, attrs, resources, roller)
		if err != nil {
			return 0, err
		}
		r, err := evalNode(n.right, attrs, resources, roller)
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
			v, err := evalNode(a, attrs, resources, roller)
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
// and at least one more digit with no intervening whitespace, folds the
// whole thing into a single DICE token (INT 'd' INT, per the grammar).
//
// Every integer literal parsed here (a bare INT, or either half of a DICE)
// goes through strconv.Atoi and its error is checked — an out-of-range
// literal (more digits than fit in an int) is a ParseError naming the
// literal's position, never a silently-clamped value.
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

	if l.pos < len(l.src) && l.src[l.pos] == 'd' {
		dPos := l.pos
		l.pos++ // consume 'd'
		sidesStart := l.pos
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
		if l.pos == sidesStart {
			return token{}, &ParseError{Pos: dPos, Msg: "invalid dice literal: expected digits after 'd'"}
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

// parseFactor := INT | DICE | ref | func | '(' expr ')'
// ref  := '@'IDENT | '#'IDENT
// func := ('floor'|'max'|'min'|'half') '(' expr (',' expr)* ')'
//
// NOTE: there is deliberately no unary '-' (or '+') case here — the
// grammar is closed and has no unary-operator production (see expr.go's
// top doc comment and TestParseRejectsUnaryMinus).
func (p *parser) parseFactor() (*Node, error) {
	switch p.tok.kind {
	case tokInt:
		n := &Node{kind: nodeInt, intValue: p.tok.intVal}
		return n, p.advance()

	case tokDice:
		n := &Node{kind: nodeDice, diceN: p.tok.diceN, diceSides: p.tok.diceSides}
		return n, p.advance()

	case tokAt:
		pos := p.tok.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent {
			return nil, &ParseError{Pos: pos, Msg: "expected identifier after '@'"}
		}
		n := &Node{kind: nodeAttrRef, ident: p.tok.text}
		return n, p.advance()

	case tokHash:
		pos := p.tok.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent {
			return nil, &ParseError{Pos: pos, Msg: "expected identifier after '#'"}
		}
		n := &Node{kind: nodeResourceRef, ident: p.tok.text}
		return n, p.advance()

	case tokIdent:
		return p.parseFuncCall()

	case tokLParen:
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tokRParen {
			return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expected ')', found %s", describeTok(p.tok))}
		}
		return inner, p.advance()

	default:
		return nil, &ParseError{Pos: p.tok.pos, Msg: fmt.Sprintf("expected expression, found %s", describeTok(p.tok))}
	}
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
