"""Independent QA of the TypeScript mutation gate: an adjudication is keyed by its code.

Derived from the requirements VTT-298..VTT-304, the ticket, and the plan's decisions
D1-D8 as amended by D15, D16, D18 and D19; the gate is loaded by import only.
"""

import collections
import copy
import importlib.util
import io
import json
import os
import re
import tempfile
import unittest

GATE_PATH = os.environ.get(
    "CHECK_TS_MUTATION_PATH",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-ts-mutation.py"),
)


def _load_gate():
    spec = importlib.util.spec_from_file_location("check_ts_mutation_under_qa", GATE_PATH)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


gate = _load_gate()

REPO = os.path.dirname(os.path.dirname(os.path.abspath(GATE_PATH)))
REAL_EQUIVALENTS = os.path.join(REPO, "tools", "ts-mutation-equivalents.txt")
REAL_REPORT = os.environ.get(
    "QA_TS_MUTATION_REPORT",
    os.path.join(REPO, "reports", "mutation", "mutation.json"),
)


def _report_is_current():
    """Stryker's report describes the tree only while its stamp's input hash is
    the tree's; a stale one would red these tests over a client change Stryker
    has not yet measured, so they skip instead. tools/ts-mutation-report.py is
    the one reader of the stamp."""
    if not os.path.exists(REAL_REPORT):
        return False
    if os.environ.get("QA_TS_MUTATION_REPORT"):
        return True
    spec = importlib.util.spec_from_file_location(
        "ts_mutation_report_for_qa", os.path.join(REPO, "tools", "ts-mutation-report.py"))
    report = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(report)
    return report.report_state(REPO)[0] == "current"

A = "client/src/a.ts"
B = "client/src/b.ts"
FILLER = "client/src/qa-filler.ts"
REASON = "QA fixture: equivalent by construction."
NO_END = object()

# The fixture every single-file scenario starts from. Line 2 is the adjudicated line;
# its mutants sit on `a > b`, which is 5 UTF-16 units long (the default span below).
BASE = [
    "export function cmp(a: number, b: number, c: number): number {",
    "  if (a > b) {",
    "    return 1;",
    "  }",
    "  if (b > c) {",
    "    return 2;",
    "  }",
    "  return 0;",
    "}",
]
CMP = BASE[1]
CMP_CODE = "if (a > b) {"
OTHER = BASE[4]

# The adjudicated line stands twice (lines 2 and 5).
DUP = [
    "export function twice(a: number, b: number): number {",
    "  if (a > b) {",
    "    return 1;",
    "  }",
    "  if (a > b) {",
    "    return 2;",
    "  }",
    "  return 0;",
    "}",
]

# The adjudicated line stands three times (lines 2, 5 and 8), each indented differently.
TRIPLE = [
    "export function thrice(a: number, b: number): number {",
    "  if (a > b) {",
    "    return 1;",
    "  }",
    "    if (a > b) {",
    "      return 2;",
    "    }",
    "\tif (a > b) {",
    "\t\treturn 3;",
    "\t}",
    "  return 0;",
    "}",
]

# The shape of the real auth.ts pair: two ConditionalExpression `false` mutants that start
# at one column, one over the whole test and one over its left operand.
G1_LINE = '  return t === null || t === "" ? null : t;'
G1_WHOLE = 't === null || t === ""'
G1_LEFT = "t === null"


# --------------------------------------------------------------------------
# Fixture arithmetic, written from D1 and D16 alone.

def u16(s):
    """Length of `s` in UTF-16 code units, which is how Stryker counts a column."""
    return len(s.encode("utf-16-le")) // 2


def indent_of(raw):
    return raw[: len(raw) - len(raw.lstrip(" \t\r"))]


def code_of(raw):
    return raw.strip(" \t\r")


def col(raw, needle, nth=0):
    """Stryker's 1-based UTF-16 column of the nth occurrence of `needle` in the raw line."""
    i = -1
    for _ in range(nth + 1):
        i = raw.index(needle, i + 1)
    return 1 + u16(raw[:i])


def off(raw, needle, nth=0):
    """D1: the entry's offset is Stryker's column minus the width of the removed indentation."""
    return col(raw, needle, nth) - u16(indent_of(raw))


def abs_index(source, line, column):
    """The UTF-16 index of a 1-based (line, column) in `source`, its terminators included."""
    lines = source.split("\n")
    return sum(u16(l) + 1 for l in lines[: line - 1]) + column - 1


def span_between(source, start, end):
    """D16 as amended after QA round 2: the UTF-16 units from start to end
    (exclusive); across lines, each line's indentation and trailing blanks are
    left out and each line break counts one unit."""
    if start[0] == end[0]:
        return abs_index(source, *end) - abs_index(source, *start)
    lines = source.split("\n")
    blanks = " \t\r"

    def cut(s, a, b):
        return s.encode("utf-16-le")[2 * a:2 * b].decode("utf-16-le")

    first = lines[start[0] - 1]
    pieces = ([cut(first, start[1] - 1, u16(first)).rstrip(blanks)]
              + [l.strip(blanks) for l in lines[start[0]:end[0] - 1]]
              + [cut(lines[end[0] - 1], 0, end[1] - 1).lstrip(blanks)])
    return u16("\n".join(pieces))


def mutant(mutator, status, replacement, line, column, span=5, end=None):
    loc = {"start": {"line": line, "column": column}}
    if end is None:
        loc["end"] = {"line": line, "column": column + span}
    elif end is not NO_END:
        loc["end"] = {"line": end[0], "column": end[1]}
    return {"mutatorName": mutator, "status": status, "replacement": replacement, "location": loc}


def entry_block(rel, mutator, offset, copies, replacement, code, reason=REASON, span=5, nth=1):
    return "%s  %s  offset=%d  span=%d  nth=%d  copies=%d  %s\n    code: %s\n    %s\n\n" % (
        rel, mutator, offset, span, nth, copies, replacement, code, reason)


def write_file(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)


def build_report(sources, mutants_by_file):
    files = {}
    for rel, ms in mutants_by_file.items():
        f = {"language": "typescript", "mutants": list(ms)}
        if sources.get(rel) is not None:
            f["source"] = sources[rel]
        files[rel] = f
    # Six killed mutants in a file of their own, so every report measures something and
    # no fixture's timeouts exceed half of what was measured (harness contract).
    files[FILLER] = {
        "language": "typescript",
        "source": "export const filler = true;\n",
        "mutants": [mutant("BooleanLiteral", "Killed", "false", 1, 23, span=4) for _ in range(6)],
    }
    return {"files": files}


Result = collections.namedtuple("Result", "rc text faults entries")


class Scn:
    """One scenario: a tree under a temp root, a hand-built Stryker report, an equivalents file."""

    def __init__(self):
        self.tree = {}
        self.sources = {}
        self.ms = collections.OrderedDict()
        self.blocks = []

    def file(self, rel, lines, nl="\n", tree_lines=None, in_tree=True, with_source=True):
        text = nl.join(lines) + nl
        if in_tree:
            self.tree[rel] = text if tree_lines is None else nl.join(tree_lines) + nl
        self.sources[rel] = text if with_source else None
        self.ms.setdefault(rel, [])
        return self

    def mut(self, rel, mutator, status, replacement, line, column, span=5, end=None):
        self.ms.setdefault(rel, []).append(mutant(mutator, status, replacement, line, column, span, end))
        return self

    def ent(self, rel, mutator, offset, copies, replacement, code, reason=REASON, span=5, nth=1):
        self.blocks.append(entry_block(rel, mutator, offset, copies, replacement, code, reason, span, nth))
        return self

    def run(self, report=None, equivalents=None):
        with tempfile.TemporaryDirectory() as root:
            for rel, text in self.tree.items():
                write_file(os.path.join(root, rel), text)
            eq = os.path.join(root, "qa-ts-equivalents.txt")
            write_file(eq, "".join(self.blocks) if equivalents is None else equivalents)
            entries = gate.read_equivalents(eq)
            faults = gate.entry_faults(entries, root)
            rep = report if report is not None else build_report(self.sources, self.ms)
            out, err = io.StringIO(), io.StringIO()
            rc = gate.check(rep, entries, out=out, err=err, root=root)
        return Result(rc, out.getvalue() + err.getvalue(), faults, entries)


def survived(text):
    return text.count("SURVIVED ")


def stale(text):
    return text.count("no longer survives")


def faulted(result):
    return [e for e, _ in result.faults]


def read_text(root, text):
    p = os.path.join(root, "qa-ts-equivalents.txt")
    write_file(p, text)
    return gate.read_equivalents(p)


# D19: the printed entry starts in column 1; its code and reason lines are indented four spaces.
PRINTED_HEADER = re.compile(r"^\S+  \S+  offset=\d+  span=\d+  nth=\d+  copies=\d+  \S.*$")


def printed_blocks(text):
    """The (header, code, reason) lines printed after each SURVIVED line, exactly as printed."""
    lines = text.splitlines()
    blocks = []
    for i, line in enumerate(lines):
        if "SURVIVED " not in line:
            continue
        for j in range(i + 1, min(i + 4, len(lines) - 2)):
            if PRINTED_HEADER.match(lines[j]):
                blocks.append(tuple(lines[j:j + 3]))
                break
    return blocks


def pasted(blocks, reason=REASON):
    """The printed blocks pasted verbatim, each placeholder reason replaced by a written one."""
    out = []
    for header, code, placeholder in blocks:
        indent = placeholder[: len(placeholder) - len(placeholder.lstrip())]
        out.append("%s\n%s\n%s%s\n\n" % (header, code, indent, reason))
    return "".join(out)


# --------------------------------------------------------------------------
# VTT-298: claimed wherever the code stands, however indented.

class ClaimedByCode(unittest.TestCase):

    # VTT-298
    def test_survivor_is_claimed_by_entry_naming_its_file_mutator_replacement_and_code(self):
        self._base()

    def _base(self, entry_offset=None, entry_code=CMP_CODE, entry_replacement="a >= b"):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, col(CMP, "a > b"))
        o = off(CMP, "a > b") if entry_offset is None else entry_offset
        s.ent(A, "EqualityOperator", o, 1, entry_replacement, entry_code)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("zero unadjudicated survivors", r.text)

    # VTT-298; ticket Done 2, "the gate exits 0 without printing `ADJUDICATION MOVED`"
    def test_survivor_on_a_line_moved_by_inserted_lines_is_claimed(self):
        for inserted in (1, 7, 300):
            with self.subTest(inserted=inserted):
                self._moved(inserted)

    def _moved(self, inserted, moved_line=CMP):
        above = ["  // QA: inserted line %d above the adjudicated mutant" % i for i in range(inserted)]
        lines = BASE[:1] + above + [moved_line] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2 + inserted, col(moved_line, "a > b"))
        # The entry is the one written when the code stood on line 2.
        s.ent(A, "EqualityOperator", off(CMP, "a > b"), 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotIn("ADJUDICATION MOVED", r.text)
        self.assertEqual(survived(r.text), 0)

    # VTT-298; D1 (re-indenting a line leaves its key unchanged)
    def test_survivor_on_a_reindented_line_is_claimed(self):
        for indent in ("", "        ", "\t\t", " \t \t"):
            with self.subTest(indent=repr(indent)):
                self._reindented(indent)

    def _reindented(self, indent, entry_offset=None):
        raw = indent + CMP_CODE
        lines = BASE[:1] + [raw] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, col(raw, "a > b"))
        # Entry written against the original two-space indentation.
        o = off(CMP, "a > b") if entry_offset is None else entry_offset
        s.ent(A, "EqualityOperator", o, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D1 (the terminator is dropped; leading and trailing spaces, tabs and CRs are removed)
    def test_crlf_and_trailing_spaces_and_tabs_are_not_part_of_the_code(self):
        self._crlf()

    def _crlf(self, trailing="  \t"):
        raw = CMP + trailing
        lines = BASE[:1] + [raw] + BASE[2:]
        s = Scn().file(A, lines, nl="\r\n")
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, col(raw, "a > b"))
        s.ent(A, "EqualityOperator", off(CMP, "a > b"), 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D1 (the interior of the line is kept as it is)
    def test_interior_whitespace_is_part_of_the_code(self):
        wide = "  if (a  > b) {"
        lines = BASE[:1] + [wide] + BASE[2:] + ["  if (a > b) {"]
        # An entry naming the single-space code does not claim the survivor on the double-space line.
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, col(wide, "a  > b"), span=6)
        s.mut(A, "ConditionalExpression", "Killed", "true", len(lines), col(CMP, "a > b"))
        s.ent(A, "ConditionalExpression", 5, 1, "true", CMP_CODE, span=6)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)
        # The entry naming the double-space code claims it.
        s2 = Scn().file(A, lines)
        s2.mut(A, "ConditionalExpression", "Survived", "true", 2, col(wide, "a  > b"), span=6)
        s2.ent(A, "ConditionalExpression", 5, 1, "true", "if (a  > b) {", span=6)
        r2 = s2.run()
        self.assertEqual(r2.faults, [])
        self.assertEqual(r2.rc, 0, r2.text)

    # D1 (runs of whitespace in Stryker's replacement collapse to one space)
    def test_replacement_whitespace_runs_are_collapsed(self):
        self._collapsed()

    def _collapsed(self, stryker_replacement="gx\n      -\t  gy"):
        raw = "  const d = gx + gy;"
        lines = BASE[:7] + [raw] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, "ArithmeticOperator", "Survived", stryker_replacement, 8, col(raw, "gx + gy"), span=7)
        s.ent(A, "ArithmeticOperator", off(raw, "gx + gy"), 1, "gx - gy", "const d = gx + gy;", span=7)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D1 (Stryker counts a column in UTF-16 units: an em dash is one)
    def test_offset_after_an_em_dash_counts_utf16_units(self):
        self._non_ascii('    if (label === "— é" || a > b) {', off)

    # D1 (Stryker counts a column in UTF-16 units: an emoji is two)
    def test_offset_after_an_emoji_counts_utf16_units(self):
        self._non_ascii('    if (label === "\U0001F600" || a > b) {', off)

    def _non_ascii(self, raw, entry_off):
        lines = BASE[:1] + [raw] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, col(raw, "a > b"))
        s.ent(A, "EqualityOperator", entry_off(raw, "a > b"), 1, "a >= b", code_of(raw))
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D6 (a StringLiteral's offset must open a string); D1 UTF-16 counting
    def test_string_literal_after_an_emoji_passes_the_static_check_and_is_claimed(self):
        self._string_after_emoji(off)

    def _string_after_emoji(self, entry_off):
        raw = '\tconst s = pick("\U0001F600", "x");'
        lines = BASE[:7] + [raw] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, "StringLiteral", "Survived", '""', 8, col(raw, '"x"'), span=3)
        s.ent(A, "StringLiteral", entry_off(raw, '"x"'), 1, '""', code_of(raw), span=3)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)


# --------------------------------------------------------------------------
# VTT-299: not claimed by an entry whose code it does not sit on.

class NotClaimedOffItsCode(unittest.TestCase):

    # VTT-299; ticket Done 4, "a survivor of the entry's mutator on another statement of the same file"
    def test_survivor_on_another_statement_with_same_mutator_replacement_and_offset_is_not_claimed(self):
        self._other_statement(5)

    def _other_statement(self, survivor_line):
        s = Scn().file(A, BASE)
        raw = BASE[survivor_line - 1]
        s.mut(A, "ConditionalExpression", "Survived", "true", survivor_line, col(raw, raw.strip()[4]))
        s.mut(A, "ConditionalExpression", "Killed", "true", 2, col(CMP, "a > b"))
        s.ent(A, "ConditionalExpression", off(CMP, "a > b"), 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED ConditionalExpression at %s:5:7 (-> true)" % A, r.text)
        self.assertEqual(stale(r.text), 1)
        self.assertEqual(r.rc, 1)

    # VTT-299; ticket Done 3, "A survivor at an entry's position whose code is not the entry's
    # is not claimed, and the entry is reported." (ConditionalExpression)
    def test_conditional_expression_entry_does_not_claim_edited_code_at_its_old_position(self):
        self._edited_at_old_position("  if (a > c) {")

    def _edited_at_old_position(self, edited):
        # The adjudicated code still stands once, further down, with its mutant killed, so the
        # static check has nothing to refuse and only the claim decides.
        lines = BASE[:1] + [edited] + BASE[2:] + [CMP]
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, col(edited, edited.strip()[4]))
        s.mut(A, "ConditionalExpression", "Killed", "true", len(lines), col(CMP, "a > b"))
        s.ent(A, "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED ConditionalExpression at %s:2:7 (-> true)" % A, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-299; ticket Done 3 (the natural case: the old code stands nowhere any more)
    def test_conditional_expression_entry_whose_code_was_edited_away_claims_nothing_and_is_reported(self):
        edited = "  if (a > c) {"
        lines = BASE[:1] + [edited] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, 7)
        s.ent(A, "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(len(r.faults), 1)
        self.assertIn(CMP_CODE, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-299 (StringLiteral: the replacement does not carry the code)
    def test_string_literal_entry_does_not_claim_edited_code_at_its_old_position(self):
        self._string_edited('  const s = "b";')

    def _string_edited(self, edited):
        old = '  const s = "a";'
        lines = BASE[:7] + [edited] + BASE[7:] + [old]
        s = Scn().file(A, lines)
        s.mut(A, "StringLiteral", "Survived", '""', 8, col(edited, '"'), span=3)
        s.mut(A, "StringLiteral", "Killed", '""', len(lines), col(old, '"'), span=3)
        s.ent(A, "StringLiteral", off(old, '"'), 1, '""', code_of(old), span=3)
        r = s.run()
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-298 (the entry names its mutator); ticket Done 4, "a survivor of another mutator at the entry's code"
    def test_survivor_of_another_mutator_with_the_same_replacement_at_the_entrys_code_is_not_claimed(self):
        self._other_mutator("BooleanLiteral")

    def _other_mutator(self, survivor_mutator):
        raw = "  if (ok) {"
        lines = BASE[:7] + [raw, "  }"] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, survivor_mutator, "Survived", "true", 8, col(raw, "ok"), span=2)
        s.mut(A, "ConditionalExpression", "Killed", "true", 8, col(raw, "ok"), span=2)
        s.ent(A, "ConditionalExpression", off(raw, "ok"), 1, "true", "if (ok) {", span=2)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-298 (for TypeScript, the entry names its replacement); ticket Done 4, "a TypeScript survivor
    # with another replacement at the entry's code"
    def test_true_and_false_at_one_place_are_two_claims(self):
        s = Scn().file(A, BASE)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, 7)
        s.mut(A, "ConditionalExpression", "Survived", "false", 2, 7)
        s.ent(A, "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED ConditionalExpression at %s:2:7 (-> false)" % A, r.text)
        self.assertEqual(r.rc, 1)
        s.ent(A, "ConditionalExpression", 5, 1, "false", CMP_CODE)
        r2 = s.run()
        self.assertEqual(r2.faults, [])
        self.assertEqual(r2.rc, 0, r2.text)

    # VTT-298 (the entry names its file)
    def test_entry_naming_another_file_with_the_same_code_does_not_claim(self):
        s = Scn().file(A, BASE).file(B, BASE)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, 7)
        s.mut(B, "ConditionalExpression", "Killed", "true", 2, 7)
        s.ent(B, "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # D1 (offset is in the key); ticket Done 4, "two mutants of one mutator on one line, as two claims"
    def test_two_mutants_of_one_mutator_and_one_replacement_on_one_line_are_two_claims(self):
        raw = "  if (a > b && b > c) {"
        lines = BASE[:7] + [raw, "  }"] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 8, col(raw, "a > b"))
        s.mut(A, "ConditionalExpression", "Survived", "true", 8, col(raw, "b > c"))
        s.ent(A, "ConditionalExpression", off(raw, "a > b"), 1, "true", code_of(raw))
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED ConditionalExpression at %s:8:%d (-> true)" % (A, col(raw, "b > c")), r.text)
        self.assertEqual(r.rc, 1)
        s.ent(A, "ConditionalExpression", off(raw, "b > c"), 1, "true", code_of(raw))
        r2 = s.run()
        self.assertEqual(r2.faults, [])
        self.assertEqual(r2.rc, 0, r2.text)

    # D16; D1 (offset and span are two fields of the key, not interchangeable)
    def test_offset_and_span_are_separate_fields_of_the_key(self):
        self._offset_and_span()

    def _offset_and_span(self, mutant_span=9):
        raw = "  if (alpha > b) {"
        lines = BASE[:7] + [raw, "  }"] + BASE[7:]
        o = off(raw, "alpha > b")
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 8, col(raw, "alpha > b"), span=mutant_span)
        s.ent(A, "ConditionalExpression", mutant_span, 1, "true", code_of(raw), span=o)  # the two numbers swapped
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)
        s2 = Scn().file(A, lines)
        s2.mut(A, "ConditionalExpression", "Survived", "true", 8, col(raw, "alpha > b"), span=9)
        s2.ent(A, "ConditionalExpression", o, 1, "true", code_of(raw), span=9)
        r2 = s2.run()
        self.assertEqual(r2.rc, 0, r2.text)

    # D16, "`span` is part of the TypeScript key" (QA's G1: one start, one mutator, one
    # replacement, two extents)
    def test_two_mutants_sharing_start_mutator_and_replacement_but_not_extent_are_two_claims(self):
        self._g1(whole_span=u16(G1_WHOLE))

    def _g1(self, whole_span):
        lines = BASE[:7] + [G1_LINE] + BASE[7:]
        start = col(G1_LINE, G1_WHOLE)
        o = off(G1_LINE, G1_WHOLE)
        left = u16(G1_LEFT)

        def scenario(whole_status, left_status, entry_spans):
            s = Scn().file(A, lines)
            s.mut(A, "ConditionalExpression", whole_status, "false", 8, start, span=whole_span)
            s.mut(A, "ConditionalExpression", left_status, "false", 8, start, span=left)
            for sp in entry_spans:
                s.ent(A, "ConditionalExpression", o, 1, "false", code_of(G1_LINE), span=sp)
            return s.run()

        # The left operand survives and is adjudicated: claimed.
        r = scenario("Killed", "Survived", [left])
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)
        # The outcomes swap: the entry does not excuse the whole-test mutant nobody judged.
        r = scenario("Survived", "Killed", [left])
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)
        # Both survive: two entries, one per extent, are two claims and not a duplicate.
        r = scenario("Survived", "Survived", [left, u16(G1_WHOLE)])
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)
        r = scenario("Survived", "Survived", [left])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # D16, "The static check cannot verify `span` against the tree without a parser. A wrong
    # `span` matches nothing and fails loudly, as stale beside unadjudicated."
    def test_a_wrong_span_is_not_refused_statically_and_claims_nothing(self):
        self._wrong_span(6)

    def _wrong_span(self, entry_span):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE, span=entry_span)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)


# --------------------------------------------------------------------------
# D16: the span, counted in the report's own source.

class Span(unittest.TestCase):

    # D16 as amended: across lines, indentation and trailing blanks are left out and a break counts one
    def test_a_multi_line_span_counts_across_the_line_terminator(self):
        for nl in ("\n", "\r\n"):
            with self.subTest(nl=repr(nl)):
                self._multi_line(nl)

    def _multi_line(self, nl, entry_span=None):
        first, second = "  if (a.x", "      > b.x) {"
        lines = BASE[:7] + [first, second, "  }"] + BASE[7:]
        source = nl.join(lines) + nl
        start, end = (8, col(first, "a.x")), (9, col(second, ")"))
        span = span_between(source, start, end)
        self.assertEqual(span, u16("a.x\n> b.x"))
        s = Scn().file(A, lines, nl=nl)
        s.mut(A, "ConditionalExpression", "Survived", "true", start[0], start[1], end=end)
        s.ent(A, "ConditionalExpression", off(first, "a.x"), 1, "true", code_of(first),
              span=span if entry_span is None else entry_span(nl))
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D16 (the span is counted in UTF-16 units)
    def test_a_span_counts_utf16_units(self):
        self._utf16_span(u16('"\U0001F600"'))

    def _utf16_span(self, entry_span):
        raw = '  const s = "\U0001F600";'
        lines = BASE[:7] + [raw] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, "StringLiteral", "Survived", '""', 8, col(raw, '"'), span=4)
        s.ent(A, "StringLiteral", off(raw, '"'), 1, '""', code_of(raw), span=entry_span)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)


# --------------------------------------------------------------------------
# VTT-300 / VTT-301 / D3 / D15: counts over one key, and one key per copy.

class MatchingCounts(unittest.TestCase):

    def _copies(self, statuses, nths, copies=2, lines=DUP):
        """Mutants of one shape on each copy of the adjudicated line; one entry per nth."""
        s = Scn().file(A, lines)
        rows = [i + 1 for i, l in enumerate(lines) if code_of(l) == CMP_CODE]
        for line_no, status in zip(rows, statuses):
            raw = lines[line_no - 1]
            s.mut(A, "EqualityOperator", status, "a >= b", line_no, col(raw, "a > b"))
        for n in nths:
            s.ent(A, "EqualityOperator", 5, copies, "a >= b", CMP_CODE, nth=n)
        return s.run()

    # VTT-300; ticket Done 4, "a second copy of an adjudicated line in the same file: one entry
    # claims one survivor, and the other survivor is reported unadjudicated"; D15 (which one)
    def test_one_entry_claims_one_of_two_survivors_on_identical_lines(self):
        self._one_entry_two_survivors()

    def _one_entry_two_survivors(self, statuses=("Survived", "Survived")):
        r = self._copies(statuses, [1])
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED EqualityOperator at %s:5:7 (-> a >= b)" % A, r.text)
        self.assertEqual(stale(r.text), 0)
        self.assertEqual(r.rc, 1)

    # D15 (an entry on one copy never claims a mutant on another copy: the Go review's case)
    def test_an_entry_on_copy_one_does_not_claim_a_survivor_on_copy_two_while_copy_ones_mutant_is_killed(self):
        self._other_copy_survives()

    def _other_copy_survives(self, statuses=("Killed", "Survived")):
        r = self._copies(statuses, [1])
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED EqualityOperator at %s:5:7 (-> a >= b)" % A, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # D15 (one entry per copy: each claims its own copy's survivor)
    def test_one_entry_per_copy_claims_both_copies(self):
        r = self._copies(("Survived", "Survived"), [1, 2])
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # VTT-301; D15 (the entry on the copy whose mutant was killed claims nothing)
    def test_an_entry_on_a_copy_whose_mutant_was_killed_is_stale(self):
        self._copy_killed()

    def _copy_killed(self, statuses=("Survived", "Killed")):
        r = self._copies(statuses, [1, 2])
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 0, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # D15 (nth counts copies from the top of the file, whatever their indentation)
    def test_nth_counts_copies_from_the_top_of_the_file(self):
        self._nth_from_top(survivor_copy=3)

    def _nth_from_top(self, survivor_copy):
        statuses = ["Killed", "Killed", "Killed"]
        statuses[survivor_copy - 1] = "Survived"
        r = self._copies(statuses, [3], copies=3, lines=TRIPLE)
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)
        r = self._copies(statuses, [2], copies=3, lines=TRIPLE)
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(stale(r.text), 1, r.text)

    # D3 (per key: survivors before timed-out mutants); two identical mutants on one copy share a key
    def test_a_survivor_is_claimed_before_a_timeout_on_the_same_key(self):
        for order in (("Survived", "Timeout"), ("Timeout", "Survived")):
            with self.subTest(order=order):
                self._survivor_before_timeout(order)

    def _survivor_before_timeout(self, statuses):
        s = Scn().file(A, BASE)
        for status in statuses:
            s.mut(A, "EqualityOperator", status, "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 0, r.text)
        self.assertEqual(stale(r.text), 0, r.text)
        self.assertEqual(r.rc, 0, r.text)

    # VTT-300; D3 (per key: one entry, two identical survivors)
    def test_one_entry_claims_one_of_two_identical_survivors_on_one_copy(self):
        self._identical_survivors(("Survived", "Survived"))

    def _identical_survivors(self, statuses):
        s = Scn().file(A, BASE)
        for status in statuses:
            s.mut(A, "EqualityOperator", status, "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # D15; D3 (copy 1 only timed out: its entry claims the timeout, never copy 2's survivor)
    def test_an_entry_on_copy_one_claims_its_timeout_and_not_copy_twos_survivor(self):
        self._timeout_on_copy_one(("Timeout", "Survived"))

    def _timeout_on_copy_one(self, statuses):
        r = self._copies(statuses, [1])
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertIn("SURVIVED EqualityOperator at %s:5:7 (-> a >= b)" % A, r.text)
        self.assertEqual(stale(r.text), 0, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-301; D3 (an entry on a timed-out mutant is not stale)
    def test_an_entry_whose_copy_timed_out_claims_the_timeout(self):
        r = self._copies(("Survived", "Timeout"), [1, 2])
        self.assertEqual(r.faults, [])
        self.assertEqual(stale(r.text), 0, r.text)
        self.assertEqual(r.rc, 0, r.text)

    # VTT-301; D3 (an entry on a timed-out mutant is not stale)
    def test_an_entry_whose_only_mutant_timed_out_is_not_stale(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Timeout", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(stale(r.text), 0, r.text)
        self.assertEqual(r.rc, 0, r.text)

    # D3 (a timed-out mutant never becomes unadjudicated)
    def test_a_timeout_with_no_entry_is_not_unadjudicated(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Timeout", "a >= b", 2, 7)
        r = s.run()
        self.assertEqual(survived(r.text), 0, r.text)
        self.assertEqual(r.rc, 0, r.text)

    # VTT-301; D3; D15 (three copies: survivor, timeout, killed — the third entry is stale)
    def test_three_entries_one_survivor_one_timeout_leave_one_stale(self):
        r = self._copies(("Survived", "Timeout", "Killed"), [1, 2, 3], copies=3, lines=TRIPLE)
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 0, r.text)
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-301
    def test_an_entry_whose_mutant_was_killed_fails_the_gate(self):
        self._killed()

    def _killed(self, status="Killed"):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", status, "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(stale(r.text), 1, r.text)
        self.assertIn(CMP_CODE, r.text)
        self.assertEqual(r.rc, 1)


# --------------------------------------------------------------------------
# VTT-302 / D4 / D6 / D15: the static check against the tree.

class StaticCheck(unittest.TestCase):

    def _faults(self, tree, blocks):
        with tempfile.TemporaryDirectory() as root:
            for rel, lines in tree.items():
                write_file(os.path.join(root, rel), "\n".join(lines) + "\n")
            entries = read_text(root, "".join(blocks))
            return entries, gate.entry_faults(entries, root)

    # VTT-302 (copies too low); the entry still matches a survivor, so only the refusal fails the gate
    def test_copies_lower_than_the_tree_count_is_refused(self):
        self._copies_low()

    def _copies_low(self, copies=1):
        s = Scn().file(A, DUP)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.mut(A, "EqualityOperator", "Killed", "a >= b", 5, 7)
        s.ent(A, "EqualityOperator", 5, copies, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(len(r.faults), 1)
        self.assertEqual(r.rc, 1, r.text)

    # VTT-302 (copies too high)
    def test_copies_higher_than_the_tree_count_is_refused(self):
        self._copies_high()

    def _copies_high(self, copies=2):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, copies, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(len(r.faults), 1)
        self.assertEqual(r.rc, 1, r.text)

    # D4 (copies counts lines whose trimmed code equals the entry's, not lines containing it)
    def test_copies_counts_lines_whose_code_equals_the_entrys_not_lines_containing_it(self):
        tree = {A: [
            "export function f(a: number, b: number): string {",
            "  if (a > b) {",
            "    return \"x\";",
            "  }",
            "\t\t\tif (a > b) {  \t",
            "  }",
            "  else if (a > b) {",
            "  }",
            "  const t = \"if (a > b) {\";",
            "  // if (a > b) {",
            "  return t;",
            "}",
        ]}
        for copies, refused in ((1, True), (2, False), (3, True), (4, True), (5, True)):
            with self.subTest(copies=copies):
                _, faults = self._faults(tree, [entry_block(A, "ConditionalExpression", 5, copies, "true", CMP_CODE)])
                self.assertEqual(bool(faults), refused, faults)

    # VTT-302; ticket Done 4, "an entry whose code stands nowhere in its file is reported, not kept"
    def test_an_entry_whose_code_stands_nowhere_is_refused_and_reported(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Killed", "a >= b", 2, 7)
        s.ent(A, "ConditionalExpression", 5, 1, "true", "if (a > z) {")
        r = s.run()
        self.assertEqual([e.code for e in faulted(r)], ["if (a > z) {"])
        self.assertIn("if (a > z) {", r.text)
        self.assertEqual(r.rc, 1)

    # VTT-302; D6 (the static check reads the tree under root)
    def test_copies_are_counted_in_the_tree_not_in_the_report(self):
        self._tree_not_report(copies=1, expect_fault=True)
        self._tree_not_report(copies=2, expect_fault=False)

    def _tree_not_report(self, copies, expect_fault, tree=DUP):
        s = Scn().file(A, BASE, tree_lines=tree)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, copies, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(bool(r.faults), expect_fault, r.faults)
        self.assertEqual(r.rc, 1 if expect_fault else 0, r.text)

    # D15 (two entries with one key are a duplicate and are refused, whatever copies says)
    def test_a_duplicate_entry_is_refused_whatever_copies_says(self):
        self._duplicate(second_nth=1)

    def _duplicate(self, second_nth):
        one = entry_block(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        _, faults = self._faults({A: BASE}, [one, one])
        self.assertTrue(faults)
        first = entry_block(A, "EqualityOperator", 5, 2, "a >= b", CMP_CODE, nth=1)
        second = entry_block(A, "EqualityOperator", 5, 2, "a >= b", CMP_CODE, nth=second_nth)
        _, faults = self._faults({A: DUP}, [first, second])
        self.assertTrue(faults)
        other = entry_block(A, "EqualityOperator", 5, 2, "a >= b", CMP_CODE, nth=2)
        _, faults = self._faults({A: DUP}, [first, other])
        self.assertEqual(faults, [])

    # D15 (an nth above copies names no copy and is refused)
    def test_an_nth_above_copies_is_refused(self):
        self._nth_above(3)

    def _nth_above(self, nth):
        _, faults = self._faults({A: DUP}, [entry_block(A, "EqualityOperator", 5, 2, "a >= b", CMP_CODE, nth=nth)])
        self.assertEqual(len(faults), 1)
        _, faults = self._faults({A: BASE}, [entry_block(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE, nth=nth - 1)])
        self.assertEqual(len(faults), 1)

    # D15 (nth counts from 1; an nth of 0 names no copy and is not accepted)
    def test_an_nth_of_zero_is_not_accepted(self):
        self._nth_zero(0)

    def _nth_zero(self, nth):
        with tempfile.TemporaryDirectory() as root:
            write_file(os.path.join(root, A), "\n".join(BASE) + "\n")
            try:
                entries = read_text(root, entry_block(A, "ConditionalExpression", 5, 1, "true", CMP_CODE, nth=nth))
            except gate.EquivalentsError:
                return
            self.assertTrue(gate.entry_faults(entries, root), "an nth of %d was accepted" % nth)

    # D16 (the static check does not verify span against the tree)
    def test_the_static_check_does_not_verify_span(self):
        self._span_unchecked(copies=1)

    def _span_unchecked(self, copies):
        _, faults = self._faults({A: BASE}, [entry_block(A, "EqualityOperator", 5, copies, "a >= b", CMP_CODE, span=99)])
        self.assertEqual(faults, [])

    # D6 (refused when its file cannot be read)
    def test_an_entry_whose_file_cannot_be_read_is_refused(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Killed", "a >= b", 2, 7)
        s.ent("client/src/missing.ts", "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual([e.file for e in faulted(r)], ["client/src/missing.ts"])
        self.assertEqual(r.rc, 1)

    # D6 (refused when its code starts like a comment: //, /* or *)
    def test_code_that_starts_like_a_comment_is_refused(self):
        lines = BASE[:7] + ["  // a > b", "  /* a > b */", "   * a > b"] + BASE[7:]
        for code in ("// a > b", "/* a > b */", "* a > b"):
            with self.subTest(code=code):
                _, faults = self._faults({A: lines}, [entry_block(A, "ConditionalExpression", 4, 1, "true", code)])
                self.assertEqual(len(faults), 1)
        _, faults = self._faults({A: lines}, [entry_block(A, "ConditionalExpression", 5, 1, "true", CMP_CODE)])
        self.assertEqual(faults, [])

    # D6 (a NO_POSITION_RULE mutator gets no check at its offset)
    def test_a_no_position_rule_mutator_is_not_checked_at_its_offset(self):
        blocks = [entry_block(A, m, 99, 1, "true", CMP_CODE) for m in gate.NO_POSITION_RULE]
        self.assertTrue(blocks)
        _, faults = self._faults({A: BASE}, blocks)
        self.assertEqual(faults, [])

    # D6 (a StringLiteral's offset must open a string: a double quote, a single quote or a backtick)
    def test_a_string_literal_offset_must_open_a_string(self):
        lines = BASE[:7] + ['  const s = "x";', "  const t = 'y';", "  const u = `z`;"] + BASE[7:]
        for raw in lines[7:10]:
            code = code_of(raw)
            q = code[10]
            for offset, refused in ((11, False), (10, True), (12, True), (99, True)):
                with self.subTest(code=code, offset=offset):
                    _, faults = self._faults({A: lines}, [entry_block(A, "StringLiteral", offset, 1, '""', code, span=3)])
                    self.assertEqual(bool(faults), refused, (q, faults))

    # D6 (an operator mutator's offset must start the expression its replacement names, and the
    # operands on both sides of the operator must agree)
    def test_an_operator_entry_must_name_the_expression_at_its_offset(self):
        lines = BASE[:7] + ["  if (a > b && c > d) {", "  const g = gx + gy;", "  return -x;"] + BASE[7:]
        cases = [
            ("EqualityOperator", "if (a > b && c > d) {", 5, "a >= b", False),
            ("EqualityOperator", "if (a > b && c > d) {", 14, "c >= d", False),
            ("EqualityOperator", "if (a > b && c > d) {", 14, "a >= b", True),
            ("EqualityOperator", "if (a > b && c > d) {", 5, "c >= d", True),
            ("EqualityOperator", "if (a > b && c > d) {", 5, "a >= c", True),
            ("EqualityOperator", "if (a > b && c > d) {", 5, "x >= b", True),
            ("EqualityOperator", "if (a > b && c > d) {", 1, "a >= b", True),
            ("EqualityOperator", "if (a > b && c > d) {", 99, "a >= b", True),
            ("ArithmeticOperator", "const g = gx + gy;", 11, "gx - gy", False),
            ("ArithmeticOperator", "const g = gx + gy;", 11, "gx - gz", True),
            ("ArithmeticOperator", "const g = gx + gy;", 16, "gx - gy", True),
            ("UnaryOperator", "return -x;", 8, "+x", False),
            ("UnaryOperator", "return -x;", 8, "+y", True),
            ("UnaryOperator", "return -x;", 1, "+x", True),
        ]
        for mutator, code, offset, replacement, refused in cases:
            with self.subTest(mutator=mutator, offset=offset, replacement=replacement):
                _, faults = self._faults({A: lines}, [entry_block(A, mutator, offset, 1, replacement, code)])
                self.assertEqual(bool(faults), refused, faults)

    # D6 (any other mutator fails closed)
    def test_a_mutator_with_no_rule_fails_closed(self):
        lines = BASE[:7] + ["  if (a && b) {"] + BASE[7:]
        for mutator, replacement in (("LogicalOperator", "a || b"), ("QaInventedMutator", "a || b")):
            with self.subTest(mutator=mutator):
                _, faults = self._faults({A: lines}, [entry_block(A, mutator, 5, 1, replacement, "if (a && b) {", span=6)])
                self.assertEqual(len(faults), 1)

    # D6 (a refusal is reported and fails the gate, and the report is still checked)
    def test_a_refused_entry_fails_the_gate_and_the_report_is_still_checked(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.mut(A, "ConditionalExpression", "Survived", "true", 5, 7)
        s.ent(A, "EqualityOperator", 5, 2, "a >= b", CMP_CODE)  # copies wrong
        r = s.run()
        self.assertEqual(len(r.faults), 1)
        self.assertIn("SURVIVED ConditionalExpression at %s:5:7 (-> true)" % A, r.text)
        self.assertIn(CMP_CODE, r.text)  # the refusal names the entry
        self.assertEqual(r.rc, 1)

    # D6 (an entry refused statically is not also told to remove the entry)
    def test_a_refused_entry_is_not_also_told_to_remove_the_entry(self):
        # Control: an entry the static check accepts and nothing claims IS told to remove it.
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Killed", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        control = s.run()
        self.assertEqual(control.faults, [])
        self.assertIn("remove the entry", control.text)
        # The same entry, refused (copies wrong), is not.
        s2 = Scn().file(A, BASE)
        s2.mut(A, "EqualityOperator", "Killed", "a >= b", 2, 7)
        s2.ent(A, "EqualityOperator", 5, 3, "a >= b", CMP_CODE)
        r = s2.run()
        self.assertEqual(len(r.faults), 1)
        self.assertNotIn("remove the entry", r.text)
        self.assertEqual(r.rc, 1)


# --------------------------------------------------------------------------
# VTT-303 / VTT-304 / D2 / D16 / D18: the report's own source.

class ReportSource(unittest.TestCase):

    # VTT-303
    def test_survivor_is_matched_against_the_report_source_when_the_tree_has_moved_on(self):
        moved_tree = BASE[:1] + ["  // QA: added after the run %d" % i for i in range(5)] + BASE[1:]
        s = Scn().file(A, BASE, tree_lines=moved_tree)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # VTT-303; D2 (a survivor's code comes from the report's own source, never from the tree)
    def test_tree_code_at_the_survivors_position_does_not_claim_it(self):
        self._tree_not_source("  if (a > c) {")

    def _tree_not_source(self, measured_line):
        measured = BASE[:1] + [measured_line] + BASE[2:]
        s = Scn().file(A, measured, tree_lines=BASE)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, 7)
        s.ent(A, "ConditionalExpression", 5, 1, "true", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(survived(r.text), 1, r.text)
        self.assertEqual(r.rc, 1)

    # VTT-304; D2 (a survivor whose line cannot be read fails; the tree holds the line)
    def test_a_survivor_beyond_the_end_of_the_report_source_fails_even_if_the_tree_holds_the_line(self):
        self._short_source(BASE)

    def _short_source(self, measured):
        # The tree holds the adjudicated line at line 12; the measured source has 9 lines.
        tree = BASE[:1] + ["  // QA: tree-only line %d" % i for i in range(10)] + [CMP] + BASE[2:]
        s = Scn().file(A, measured, tree_lines=tree)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 12, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.rc, 1, r.text)

    # VTT-304; D2 (a report file with no source fails)
    def test_a_survivor_in_a_report_file_without_source_fails_even_if_the_tree_holds_the_line(self):
        self._no_source(False)

    def _no_source(self, with_source):
        s = Scn().file(A, BASE, with_source=with_source)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.rc, 1, r.text)

    # VTT-304 (line 0 cannot be read: Stryker's lines are 1-based, and line 0 is not the last line)
    def test_a_survivor_on_line_zero_fails_rather_than_reading_the_last_line(self):
        self._line_zero(0)

    def _line_zero(self, line):
        lines = BASE[:1] + [CMP]  # the adjudicated line is also the last line, with no newline after it
        s = Scn()
        s.tree[A] = "\n".join(lines)
        s.sources[A] = "\n".join(lines)
        s.ms[A] = [mutant("EqualityOperator", "Survived", "a >= b", line, 7)]
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.rc, 1, r.text)

    # VTT-304; D16, "A TypeScript survivor whose report gives no end, or an end outside the
    # source, fails the gate (VTT-304)."
    def test_a_survivor_with_no_end_fails_and_no_entry_is_printed_for_it(self):
        self._end(NO_END)

    # VTT-304; D16, "A TypeScript survivor whose report gives no end, or an end outside the
    # source, fails the gate (VTT-304)."
    def test_a_survivor_whose_end_lies_outside_the_source_fails_and_no_entry_is_printed_for_it(self):
        for end in ((40, 1), (len(BASE) + 1, 5)):
            with self.subTest(end=end):
                self._end(end)

    def _end(self, end, expect_rc=1):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7, end=end)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.rc, expect_rc, r.text)
        self.assertIn("%s:2:7" % A, r.text)
        self.assertEqual(printed_blocks(r.text), [])

    # VTT-304; D18 (a column in the line's indentation names no code)
    def test_a_survivor_whose_column_is_in_the_indentation_names_no_code(self):
        for column in (1, 4):
            with self.subTest(column=column):
                self._indentation(column, expect_block=False)
        # Control: the first column of the code itself is readable and prints its entry.
        self._indentation(5, expect_block=True)

    def _indentation(self, column, expect_block):
        raw = "    if (a > b) {"
        lines = BASE[:1] + [raw] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, column, span=1)
        r = s.run()
        self.assertEqual(r.rc, 1, r.text)
        self.assertIn("SURVIVED ConditionalExpression at %s:2:%d" % (A, column), r.text)
        self.assertEqual(bool(printed_blocks(r.text)), expect_block, r.text)

    # VTT-304; D18 (a column past the end of the trimmed line names no code, even beside an entry
    # whose offset it would compute)
    def test_a_survivor_whose_column_is_past_the_trimmed_line_names_no_code(self):
        raw = "  if (a > b) {   "
        last = u16(indent_of(raw)) + u16(code_of(raw))  # the column of the code's last character
        # Control: the last character of the code is readable and its entry claims it.
        self._past_end(raw, last, expect_rc=0)
        for column in (last + 1, last + 3, last + 20):
            with self.subTest(column=column):
                self._past_end(raw, column, expect_rc=1)

    def _past_end(self, raw, column, expect_rc):
        lines = BASE[:1] + [raw] + BASE[2:]
        s = Scn().file(A, lines)
        s.mut(A, "ConditionalExpression", "Survived", "true", 2, column, span=1)
        s.ent(A, "ConditionalExpression", column - u16(indent_of(raw)), 1, "true", code_of(raw), span=1)
        r = s.run()
        self.assertEqual(r.rc, expect_rc, r.text)


# --------------------------------------------------------------------------
# D5: the file format.

class FileFormat(unittest.TestCase):

    def _read(self, text):
        with tempfile.TemporaryDirectory() as root:
            return read_text(root, text)

    # D5 (a # anywhere but the first character of a line is text, in code, replacement and reason)
    def test_a_hash_inside_code_replacement_and_reason_is_text(self):
        es = self._read(
            "# a comment\n"
            "client/src/a.ts  EqualityOperator  offset=5  span=15  nth=1  copies=1  this.#count >= 0\n"
            "    code: if (this.#count > 0) { // #1\n"
            "    reason with # inside, and #hash\n")
        self.assertEqual(len(es), 1)
        self.assertEqual(es[0].replacement, "this.#count >= 0")
        self.assertEqual(es[0].code, "if (this.#count > 0) { // #1")
        self.assertEqual(es[0].reason, "reason with # inside, and #hash")
        # and the entry claims its survivor
        raw = "  if (this.#count > 0) { // #1"
        lines = BASE[:7] + [raw, "  }"] + BASE[7:]
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "this.#count >= 0", 8, col(raw, "this.#count"), span=15)
        s.ent(A, "EqualityOperator", off(raw, "this.#count"), 1, "this.#count >= 0", code_of(raw), "see #42", span=15)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)

    # D5 (a comment is a line whose first character is #)
    def test_an_indented_hash_line_is_the_reason_not_a_comment(self):
        es = self._read(
            "client/src/a.ts  ConditionalExpression  offset=5  span=5  nth=1  copies=1  true\n"
            "    code: if (a > b) {\n"
            "    # this is the reason, not a comment\n")
        self.assertEqual(len(es), 1)
        self.assertEqual(es[0].reason, "# this is the reason, not a comment")

    # D5 (a comment is a line whose first character is #); D15; D16 (the labelled fields)
    def test_comment_lines_between_entries_are_ignored(self):
        es = self._read(
            "#client/src/a.ts  ConditionalExpression  offset=5  span=5  nth=1  copies=1  true\n"
            "client/src/a.ts  ConditionalExpression  offset=5  span=7  nth=2  copies=3  true\n"
            "    code: if (a > b) {\n"
            "    first\n"
            "# ---- client/src/b.ts ----\n"
            "#    code: nothing\n"
            "client/src/b.ts  ConditionalExpression  offset=11  span=4  nth=1  copies=1  false\n"
            "    code: if (b > c) {\n"
            "    second\n")
        self.assertEqual([(e.file, e.replacement, e.code, e.reason) for e in es], [
            ("client/src/a.ts", "true", "if (a > b) {", "first"),
            ("client/src/b.ts", "false", "if (b > c) {", "second")])
        self.assertEqual([(e.mutator, e.offset, e.span, e.nth, e.copies) for e in es],
                         [("ConditionalExpression", 5, 7, 2, 3), ("ConditionalExpression", 11, 4, 1, 1)])

    # D5 (an entry without its code: line is a parse error)
    def test_an_entry_without_its_code_line_is_a_parse_error(self):
        with self.assertRaises(gate.EquivalentsError):
            self._read("client/src/a.ts  ConditionalExpression  offset=5  span=5  nth=1  copies=1  true\n"
                       "    the reason\n")

    # D5 (an entry without its reason is a parse error)
    def test_an_entry_without_its_reason_is_a_parse_error(self):
        for tail in ("", "\n",
                     "client/src/b.ts  ConditionalExpression  offset=5  span=5  nth=1  copies=1  false\n"
                     "    code: if (b > c) {\n    reason\n"):
            with self.subTest(tail=tail[:20]):
                with self.assertRaises(gate.EquivalentsError):
                    self._read("client/src/a.ts  ConditionalExpression  offset=5  span=5  nth=1  copies=1  true\n"
                               "    code: if (a > b) {\n" + tail)

    # D5 (a reason with no entry above it is a parse error)
    def test_a_reason_with_no_entry_above_it_is_a_parse_error(self):
        with self.assertRaises(gate.EquivalentsError):
            self._read("# header\n    an orphan reason\n")


# --------------------------------------------------------------------------
# D8 / D19: messages.

class Messages(unittest.TestCase):

    # D8 (the SURVIVED line, then the entry ready to paste, copies included); D19 (the header in
    # column 1, the code and reason lines indented four spaces); D15; D16
    def test_an_unadjudicated_survivor_message_names_mutator_position_replacement_and_entry(self):
        self._message(col("      if (a > b) {", "a > b"))

    def _message(self, column):
        deep = "      if (a > b) {"
        lines = BASE[:7] + [deep, "      }"] + BASE[7:]  # the second copy, deeper indent
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 8, column)
        r = s.run()
        self.assertEqual(r.rc, 1)
        self.assertIn("SURVIVED EqualityOperator at client/src/a.ts:8:11 (-> a >= b)", r.text)
        blocks = printed_blocks(r.text)
        self.assertEqual([b[:2] for b in blocks], [
            ("client/src/a.ts  EqualityOperator  offset=5  span=5  nth=2  copies=2  a >= b",
             "    code: if (a > b) {")])
        reason = blocks[0][2]
        self.assertTrue(reason.startswith("    ") and not reason.startswith("     ") and reason.strip(), reason)

    # D19 (the printed block, with its reason written, parses and claims without dedenting);
    # D1, D15, D16 throughout
    def test_the_printed_entries_pasted_verbatim_claim_every_survivor(self):
        self._round_trip()

    def _round_trip(self, corrupt=None):
        emoji = '    if (label === "\U0001F600—" || a > b) {'
        priv = "\tif (this.#count > 0) {"
        arith = "  const d = gx + gy;"
        first, second = "  if (a.x", "      > b.x) {"
        lines = (BASE[:1] + [emoji, "  }", priv, "  }", arith]
                 + ["  if (a > b) {", "  }", "    if (a > b) {", "    }"]
                 + [G1_LINE, first, second, "  }"] + BASE[7:])
        # line numbers: 2 emoji, 4 priv, 6 arith, 7 and 9 the two copies, 11 G1, 12-13 multi-line
        source = "\n".join(lines) + "\n"
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, col(emoji, "a > b"))
        s.mut(A, "StringLiteral", "Survived", '""', 2, col(emoji, '"'), span=u16('"\U0001F600—"'))
        s.mut(A, "EqualityOperator", "Survived", "this.#count >= 0", 4, col(priv, "this.#count"), span=15)
        s.mut(A, "ArithmeticOperator", "Survived", "gx\n    - gy", 6, col(arith, "gx + gy"), span=7)
        s.mut(A, "ConditionalExpression", "Survived", "true", 7, 7)
        s.mut(A, "ConditionalExpression", "Survived", "true", 9, 9)
        s.mut(A, "ConditionalExpression", "Survived", "false", 11, col(G1_LINE, G1_WHOLE), span=u16(G1_WHOLE))
        s.mut(A, "ConditionalExpression", "Survived", "false", 11, col(G1_LINE, G1_WHOLE), span=u16(G1_LEFT))
        start, end = (12, col(first, "a.x")), (13, col(second, ")"))
        s.mut(A, "ConditionalExpression", "Survived", "true", start[0], start[1], end=end)
        first_run = s.run()
        self.assertEqual(first_run.rc, 1)
        blocks = printed_blocks(first_run.text)
        self.assertEqual(len(blocks), 9, first_run.text)
        self.assertIn(("client/src/a.ts  ConditionalExpression  offset=5  span=%d  nth=1  copies=1  true"
                       % span_between(source, start, end), "    code: if (a.x"), [b[:2] for b in blocks])
        text = pasted(blocks)
        if corrupt is not None:
            text = corrupt(text)
        second_run = s.run(equivalents=text)
        self.assertEqual(second_run.faults, [])
        self.assertEqual(second_run.rc, 0, second_run.text)

    # D8 (a clean run prints zero unadjudicated survivors and returns 0)
    def test_a_clean_run_prints_zero_unadjudicated_survivors_and_returns_zero(self):
        s = Scn().file(A, BASE)
        s.mut(A, "EqualityOperator", "Killed", "a >= b", 2, 7)
        r = s.run()
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("zero unadjudicated survivors", r.text)


# --------------------------------------------------------------------------
# The real adjudication file, the real tree and Stryker's real report.

def own_key(path, source, m):
    """D1, D15 and D16, computed here independently of the gate."""
    lines = source.split("\n")
    s, e = m["location"]["start"], m["location"]["end"]
    raw = lines[s["line"] - 1]
    code = code_of(raw)
    nth = sum(1 for l in lines[: s["line"]] if code_of(l) == code)
    span = span_between(source, (s["line"], s["column"]), (e["line"], e["column"]))
    return (path, m["mutatorName"], re.sub(r"\s+", " ", m["replacement"]),
            s["column"] - u16(indent_of(raw)), span, nth, code)


def entry_tuple(e):
    return (e.file, e.mutator, e.replacement, e.offset, e.span, e.nth, e.code)


@unittest.skipUnless(_report_is_current(), "Stryker's report at %s is absent or older than the client" % REAL_REPORT)
class RealData(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        with open(REAL_REPORT, encoding="utf-8") as f:
            cls.report = json.load(f)
        cls.entries = gate.read_equivalents(REAL_EQUIVALENTS)

    def run_gate(self, report, root):
        out, err = io.StringIO(), io.StringIO()
        rc = gate.check(report, self.entries, out=out, err=err, root=root)
        return rc, out.getvalue() + err.getvalue()

    def survivors(self, report):
        return [(p, m) for p, f in report["files"].items() for m in f["mutants"] if m["status"] == "Survived"]

    # VTT-302; D6; D15 (the real file against the real tree)
    def test_the_real_file_passes_the_static_check_against_the_real_tree(self):
        self.assertEqual(gate.entry_faults(self.entries, REPO), [])

    # D4 (copies), counted here independently
    def test_every_real_entrys_copies_is_the_tree_count_of_its_code(self):
        wrong = []
        for e in self.entries:
            with open(os.path.join(REPO, e.file), encoding="utf-8", newline="") as f:
                n = sum(1 for l in f.read().split("\n") if code_of(l) == e.code)
            if n != e.copies:
                wrong.append((e.file, e.code, e.copies, n))
        self.assertEqual(wrong, [])

    # VTT-298, VTT-300, VTT-301; ticket Done 5, "the new keys claim exactly what the old keys claim"
    def test_the_real_file_claims_exactly_the_real_reports_survivors(self):
        rc, text = self.run_gate(self.report, REPO)
        self.assertEqual(rc, 0, text[-3000:])
        self.assertIn("zero unadjudicated survivors", text)
        self.assertNotIn("SURVIVED ", text)
        self.assertNotIn("no longer survives", text)
        mine = collections.Counter(own_key(p, self.report["files"][p]["source"], m) for p, m in self.survivors(self.report))
        theirs = collections.Counter(entry_tuple(e) for e in self.entries)
        self.assertEqual(sum(mine.values()), len(self.entries))
        self.assertEqual(mine - theirs, collections.Counter())
        self.assertEqual(theirs - mine, collections.Counter())

    def _auth_pair(self, report):
        """The real auth.ts mutants that share file, mutator, replacement and start (QA's G1)."""
        f = report["files"]["client/src/auth.ts"]
        groups = collections.defaultdict(list)
        for m in f["mutants"]:
            if m["mutatorName"] == "ConditionalExpression" and m["replacement"] == "false":
                s = m["location"]["start"]
                groups[(s["line"], s["column"])].append(m)
        pairs = [ms for ms in groups.values() if len(ms) == 2 and {m["status"] for m in ms} == {"Killed", "Survived"}]
        self.assertEqual(len(pairs), 1)
        return pairs[0]

    # D16 (QA's G1 on the real report: the auth.ts entry claims only the left-operand mutant)
    def test_the_real_auth_entry_claims_only_the_mutant_it_was_written_for(self):
        self._auth(swap=lambda pair: None)

    def _auth(self, swap):
        rep = copy.deepcopy(self.report)
        pair = self._auth_pair(rep)
        swap(pair)
        source = rep["files"]["client/src/auth.ts"]["source"]
        live = [m for m in pair if m["status"] == "Survived"][0]
        dead = [m for m in pair if m["status"] == "Killed"][0]
        live_key, dead_key = own_key("client/src/auth.ts", source, live), own_key("client/src/auth.ts", source, dead)
        self.assertNotEqual(live_key, dead_key)
        mine = [e for e in self.entries if entry_tuple(e)[:3] == live_key[:3] and e.code == live_key[6]]
        self.assertEqual([entry_tuple(e) for e in mine], [live_key])
        # The outcomes swap: the whole-test mutant survives, the left operand is killed.
        live["status"], dead["status"] = "Killed", "Survived"
        rc, text = self.run_gate(rep, REPO)
        where = "client/src/auth.ts:%d:%d" % (dead["location"]["start"]["line"], dead["location"]["start"]["column"])
        self.assertEqual(rc, 1)
        self.assertEqual(re.findall(r"SURVIVED \S+ at (\S+) ", text), [where])
        self.assertEqual(stale(text), 1, text[-2000:])
        self.assertIn("client/src/auth.ts ConditionalExpression", text)

    def _tree_copy(self, root, edit=None):
        """Copy every report file's tree version under root, optionally editing its lines."""
        for p in self.report["files"]:
            with open(os.path.join(REPO, p), encoding="utf-8", newline="") as f:
                text = f.read()
            if edit is not None:
                text = edit(p, text)
            write_file(os.path.join(root, p), text)

    # VTT-298; ticket Done 1, "With a comment line inserted at the top of `internal/engine/apply.go`
    # and of `client/src/fold.ts`"
    def test_real_entries_still_claim_after_a_comment_line_at_the_top_of_fold_ts(self):
        self._shifted({"client/src/fold.ts": 1})

    # VTT-298 (every adjudicated file moved, by different amounts)
    def test_real_entries_still_claim_after_every_file_moves(self):
        files = sorted({e.file for e in self.entries})
        self._shifted({p: 3 + 17 * i for i, p in enumerate(files)})

    def _shifted(self, shifts, extra=0):
        rep = copy.deepcopy(self.report)
        for p, n in shifts.items():
            f = rep["files"][p]
            f["source"] = "".join("// QA: line %d inserted at the top\n" % i for i in range(n)) + f["source"]
            for m in f["mutants"]:
                m["location"]["start"]["line"] += n + extra
                m["location"]["end"]["line"] += n + extra
        with tempfile.TemporaryDirectory() as root:
            self._tree_copy(root, lambda p, t: "".join(
                "// QA: line %d inserted at the top\n" % i for i in range(shifts.get(p, 0))) + t)
            self.assertEqual(gate.entry_faults(self.entries, root), [])
            rc, text = self.run_gate(rep, root)
        self.assertEqual(rc, 0, text[-3000:])
        self.assertNotIn("ADJUDICATION MOVED", text)

    # VTT-298 (however it is indented): every real survivor, multi-line nodes included, since
    # D16 as amended leaves each line's indentation out of the span.
    def test_real_survivors_still_claimed_after_every_file_is_reindented(self):
        self._reindented_real(u16("\t "))

    def _reindented_real(self, column_shift):
        pad = "\t "
        rep = copy.deepcopy(self.report)
        for p, f in rep["files"].items():
            f["source"] = "\n".join(pad + l for l in f["source"].split("\n"))
            for m in f["mutants"]:
                m["location"]["start"]["column"] += column_shift
                m["location"]["end"]["column"] += column_shift
        with tempfile.TemporaryDirectory() as root:
            self._tree_copy(root, lambda p, t: "\n".join(pad + l for l in t.split("\n")))
            self.assertEqual(gate.entry_faults(self.entries, root), [])
            rc, text = self.run_gate(rep, root)
        self.assertEqual(set(re.findall(r"SURVIVED \S+ at (\S+) ", text)), set(), text[-3000:])
        self.assertEqual(rc, 0, text[-3000:])

    # VTT-303 (the report measured the tree as it was; the tree has since gained a line at the top of fold.ts)
    def test_the_real_report_is_matched_against_its_own_source_when_the_tree_has_moved(self):
        with tempfile.TemporaryDirectory() as root:
            self._tree_copy(root, lambda p, t: ("// QA: added after the run\n" + t) if p == "client/src/fold.ts" else t)
            self.assertEqual(gate.entry_faults(self.entries, root), [])
            rc, text = self.run_gate(self.report, root)
        self.assertEqual(rc, 0, text[-3000:])


# --------------------------------------------------------------------------
# A line is Stryker's line.

class StrykerLines(unittest.TestCase):

    # D1 (code is the line where the mutant starts): a form feed is whitespace in TypeScript, not a
    # line terminator, so it does not move Stryker's line numbers.
    def test_a_form_feed_in_a_comment_does_not_move_the_line(self):
        lines = [BASE[0] + " // page\fbreak"] + BASE[1:]
        s = Scn().file(A, lines)
        s.mut(A, "EqualityOperator", "Survived", "a >= b", 2, 7)
        s.ent(A, "EqualityOperator", 5, 1, "a >= b", CMP_CODE)
        r = s.run()
        self.assertEqual(r.faults, [])
        self.assertEqual(r.rc, 0, r.text)


if __name__ == "__main__":
    unittest.main()
