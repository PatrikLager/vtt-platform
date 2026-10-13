#!/usr/bin/env python3
"""Boundary tests for check-ts-mutation.py.

This script decides whether a surviving mutant fails the build, so a bug in it
silently re-opens every gap mutation testing exists to find. Its Go sibling's
tests exist because a review found ways that gate could pass when it should
fail; the same shapes are pinned here.

Run: python3 tools/check_ts_mutation_test.py
"""

import importlib.util
import io
import os
import tempfile
import unittest

_spec = importlib.util.spec_from_file_location(
    "check_ts_mutation",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-ts-mutation.py"))
ctm = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(ctm)


def report(*mutants, lines=None, sources=None):
    """mutants: (file, line, col, mutator, status[, replacement[, span]]); the
    mutant ends `span` UTF-16 units after its start, on the same line.

    Each file carries its `source`, as Stryker's report does: the fixture text
    built from `lines` (TS_LINES by default), unless `sources` names it.
    """
    files = {}
    for m in mutants:
        path, line, col, mutator, status = m[:5]
        replacement = m[5] if len(m) > 5 else "REP"
        span = m[6] if len(m) > 6 else 1
        files.setdefault(path, {"mutants": []})["mutants"].append({
            "mutatorName": mutator,
            "status": status,
            "replacement": replacement,
            "location": {"start": {"line": line, "column": col},
                         "end": {"line": line, "column": col + span}},
        })
    for path, entry in files.items():
        if sources is None:
            entry["source"] = ts_text(lines or TS_LINES)
        elif path in sources:
            entry["source"] = sources[path]
    return {"files": files}


# The source every fixture below names: written to the tree as client/src/a.ts
# and b.ts, and carried in each report as the file's `source`.
#
# DELIBERATELY DISTINCT where two survivors must sit on different code (10 vs
# 14), and DELIBERATELY IDENTICAL where one code must stand several times in a
# file (30/40/50/60, 90/91).
TS_LINES = {
    9: "const nine = true;",
    10: "if (a > b) c();",          # col 5 opens `a > b`
    14: "if (x > y) z();",          # the same SHAPE, different operands
    15: "if (a > q) c();",          # the SIBLING comparison: same left operand
                                    # and same operator as line 10, one operand
                                    # apart -- the near miss that "looks
                                    # plausible at the new location"
    20: "const twenty = { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 7 };",
    21: "if (a > b) c(); // — an em dash AFTER the operator, harmless",
    22: 'const dash = "—"; if (a > b) c();',
                                    # ...and one BEFORE it, which is not. U+2014
                                    # is ONE character and THREE UTF-8 bytes, so
                                    # a byte-indexed column lands two early. In
                                    # code rather than in a leading comment, or
                                    # the comment rule answers first and this
                                    # never reaches the column rule.
    23: 'const e = "😀"; if (a > b) c();',
                                    # THE ONE THAT SEPARATES UTF-16 FROM
                                    # CHARACTERS. U+1F600 is astral: JavaScript
                                    # counts it as TWO code units, Python as one
                                    # character. Verified against @babel/parser
                                    # — it reports the BinaryExpression at
                                    # 0-based column 20, so Stryker says 21.
                                    # Character counting maps 21 to a space and
                                    # refuses a sound key; only UTF-16 lands on
                                    # the `a`. Without this line the em-dash
                                    # fixtures pass under either scheme.
    30: 'send("tag");',             # col 6 opens a string literal
    40: 'send("tag");',             # three more copies of ONE statement
    50: 'send("tag");',
    60: 'send("tag");',
    42: "const fortytwo = true;",
    70: "// a line comment",
    71: "/** a doc comment */",
    72: " * a doc-comment continuation line",
    77: "const seventyseven = true;",
    80: "const eighty = notAString;",   # col 16 is an identifier, not a quote
    90: 'label("other");',          # col 7 opens a string literal, on a
    91: 'label("other");',          # statement the `send` lines cannot be
    95: 'send("a", "b");',          # cols 6 and 11 open strings: two mutants of
                                    # one mutator, one replacement, one line
}


def ts_text(lines=TS_LINES):
    """The fixture file's text, padded so the line numbers are real."""
    body = []
    while len(body) < max(lines):
        body.append(lines.get(len(body) + 1, f"const l{len(body) + 1} = 0;"))
    return "\n".join(body) + "\n"


def source_tree(root, lines=TS_LINES):
    """client/src/a.ts and b.ts."""
    src = os.path.join(root, "client", "src")
    os.makedirs(src, exist_ok=True)
    for name in ("a.ts", "b.ts"):
        with open(os.path.join(src, name), "w") as fh:
            fh.write(ts_text(lines))
    return root


def run(rep, equivalents=None, lines=TS_LINES):
    out, err = io.StringIO(), io.StringIO()
    with tempfile.TemporaryDirectory() as root:
        source_tree(root, lines)
        code = ctm.check(rep, equivalents or [], out=out, err=err, root=root)
    return code, out.getvalue() + err.getvalue()


def equivalents_file(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


def entry(path, where, mutator, replacement, reason="reason", lines=TS_LINES, span=1):
    """One entry in the equivalents format, for the fixture code a mutant at
    `where` (`line:col`) starts on, covering `span` units as report() makes it.
    The entry names that code, the offset in it, which copy of it and its count
    in the file, and no line (SPEC-020)."""
    line, col = (int(x) for x in where.split(":"))
    raw = lines[line]
    code = raw.strip(" \t\r")
    offset = col - (len(raw) - len(raw.lstrip(" \t\r")))
    nth = sum(1 for n, v in lines.items() if n <= line and v.strip(" \t\r") == code)
    copies = sum(1 for v in lines.values() if v.strip(" \t\r") == code)
    return (f"{path}  {mutator}  offset={offset}  span={span}  nth={nth}  copies={copies}  {replacement}\n"
            f"    code: {code}\n    {reason}\n")


def entries(*texts):
    return ctm.read_equivalents(equivalents_file("".join(texts)))


class Equivalents(unittest.TestCase):
    def test_an_entry_and_its_reason_parse(self):
        [e] = entries("client/src/a.ts  EqualityOperator  offset=5  span=5  nth=1  copies=1  a >= b\n"
                      "    code: if (a > b) c();\n    same observable\n")
        self.assertEqual(e, ctm.Entry("client/src/a.ts", "EqualityOperator", 5, 5, 1, 1, "a >= b",
                                      "if (a > b) c();", "same observable"))

    def test_an_entry_without_a_reason_is_an_error(self):
        # An equivalence claim forecloses ever writing a test for that mutant;
        # a bare entry is how a wrong one lands without anyone noticing.
        with self.assertRaises(ctm.EquivalentsError):
            entries("client/src/a.ts  EqualityOperator  offset=5  span=5  nth=1  copies=1  a >= b\n"
                    "    code: if (a > b) c();\n")

    def test_an_entry_without_a_code_line_is_fatal(self):
        with self.assertRaises(ctm.EquivalentsError) as caught:
            entries("client/src/a.ts  EqualityOperator  offset=5  span=5  nth=1  copies=1  a >= b\n"
                    "    same observable\n")
        self.assertIn("code line", str(caught.exception))

    def test_a_reason_with_no_entry_is_an_error(self):
        with self.assertRaises(ctm.EquivalentsError):
            entries("    orphaned reason\n")

    def test_a_malformed_entry_is_an_error(self):
        with self.assertRaises(ctm.EquivalentsError):
            entries("client/src/a.ts  EqualityOperator  a >= b\n    code: x\n    reason\n")

    def test_the_printed_placeholder_is_refused_as_a_reason(self):
        """Pasted unedited, the ready entry would excuse a mutant nobody judged."""
        with self.assertRaises(ctm.EquivalentsError) as caught:
            entries("client/src/a.ts  EqualityOperator  offset=5  span=5  nth=1  copies=1  a >= b\n"
                    "    code: if (a > b) c();\n    <the observable that makes it equivalent>\n")
        self.assertIn("placeholder", str(caught.exception))

    def test_a_code_line_keeps_a_trailing_form_feed_and_no_break_space(self):
        """The code line is trimmed as the gate trims source, so what the gate
        prints, pasted back, names the same code."""
        [e] = entries("client/src/a.ts  StringLiteral  offset=6  span=6  nth=1  copies=1  \"\"\n"
                      "    code: send(\"x\");\u00a0\x0c\n    reason\n")
        self.assertEqual(e.code, 'send("x");\u00a0\x0c')

    def test_a_hash_in_a_replacement_or_reason_is_kept(self):
        """Only a line whose FIRST character is `#` is a comment."""
        [e] = entries("# a comment\n"
                      "client/src/a.ts  StringLiteral  offset=6  span=6  nth=1  copies=1  \"#fff\"\n"
                      "    code: send(\"#000\");\n    a # in a reason is text\n")
        self.assertEqual((e.replacement, e.code, e.reason),
                         ('"#fff"', 'send("#000");', "a # in a reason is text"))


class Gate(unittest.TestCase):
    def test_all_killed_passes(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")))
        self.assertEqual(code, 0, msg)

    def test_one_survivor_fails(self):
        code, msg = run(report(("client/src/a.ts", 7, 3, "Eq", "Killed"),
                               ("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived")))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED BooleanLiteral at client/src/a.ts:9:5", msg)

    # VTT-298
    def test_an_adjudicated_survivor_passes(self):
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "REP", "no observable difference")))
        self.assertEqual(code, 0, msg)

    # VTT-301
    def test_an_adjudication_for_a_mutant_that_no_longer_survives_fails(self):
        # Otherwise the entry silently pre-approves whatever mutant later
        # lands on that code.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Killed")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "REP", "stale")))
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)

    # VTT-301
    def test_an_adjudication_whose_mutant_timed_out_is_not_called_stale(self):
        # A TIMED OUT mutant was NOT EVALUATED, so it is no evidence that the
        # adjudication above it has expired. The gate counts a timeout as a
        # detection for the pass/fail verdict (Patrik, 2026-07-28) and that is
        # right: a mutant that hangs the suite was detected. Reusing that for
        # STALENESS is not, and it inverted into advice to delete good work.
        #
        # MEASURED 2026-08-27, which is why this test exists: three entries on
        # client/src/view/spectator.ts (364:10 and 365:10 EqualityOperator,
        # 501:59 StringLiteral) were reported stale with "Remove the entry".
        # All three were applied by hand and SURVIVED a full `bun test
        # client/test` — 685 pass, 0 fail. Following the advice would have
        # deleted three exhaustively-reasoned adjudications, and they would
        # have returned as UNADJUDICATED survivors the moment the timeouts
        # were fixed, with their reasoning gone.
        #
        # UPDATE 2026-08-30: 501:59's own REASONING has since been proven
        # false by Task 4 (dm-hands-and-retraction) — a prototype-injected
        # state makes that fallback string observable after all — and the
        # entry was deliberately withdrawn; it carries no adjudication
        # today. 364:10 and 365:10 remain sound. That does not touch what
        # this test is checking: the point was never that 501:59's claim was
        # correct, only that a TIMEOUT is not the thing that would have told
        # anyone it was wrong.
        code, msg = run(
            report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                   ("client/src/a.ts", 9, 5, "BooleanLiteral", "Timeout")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "REP", "no observable difference")))
        self.assertEqual(code, 0, msg)
        self.assertNotIn("no longer survives", msg)
        # And it is still reported as unevaluated, because suppressing the
        # staleness complaint must not also suppress the timeout itself.
        self.assertIn("timed out: client/src/a.ts:9:5", msg)

    # VTT-299
    def test_an_adjudication_is_keyed_by_mutator_not_just_location(self):
        # Two mutators at one location are two different claims; excusing one
        # must not excuse the other.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "EqualityOperator", "Survived")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "REP", "different mutator")))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED EqualityOperator", msg)

    # VTT-298
    def test_two_mutants_at_one_location_are_two_separate_claims(self):
        # Stryker emits `-> true` AND `-> false` for one ConditionalExpression.
        # Before the replacement was part of the key these collapsed into one
        # entry, so adjudicating the harmless one excused the other. Found by
        # the duplicate check on client/src/player.ts:20:51.
        rep = report(("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "true"),
                     ("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false"))
        code, msg = run(rep, entries(entry("client/src/a.ts", "20:51", "ConditionalExpression", "true", "reached only when true")))
        self.assertEqual(code, 1)
        self.assertIn("-> false", msg)

    def test_a_timed_out_mutant_counts_as_a_kill_but_is_printed(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Timeout")))
        self.assertEqual(code, 0, msg)
        self.assertIn("timed out: client/src/a.ts:2:1", msg)

    def test_a_majority_of_timeouts_is_rejected_as_an_unusable_measurement(self):
        # Counting timeouts as kills inverts into scoring a broken run as
        # perfect — observed on the Go side at 58 of 64.
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Timeout"),
                               ("client/src/a.ts", 3, 1, "Eq", "Timeout")))
        self.assertEqual(code, 1)
        self.assertIn("timed out", msg)

    def test_exactly_half_timing_out_is_accepted(self):
        # The boundary is pinned by test rather than left to the reader.
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Timeout")))
        self.assertEqual(code, 0, msg)

    def test_an_empty_report_fails_rather_than_passing_vacuously(self):
        code, msg = run({"files": {}})
        self.assertEqual(code, 1)
        self.assertIn("no mutants were measured", msg)

    def test_a_report_of_only_unbuildable_mutants_fails(self):
        # CompileError is not a detection in either direction; a run that
        # produced nothing else measured nothing.
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "CompileError")))
        self.assertEqual(code, 1)
        self.assertIn("no mutants were measured", msg)

    def test_no_coverage_is_an_error_because_the_command_runner_cannot_produce_it(self):
        # Every mutant is handed the whole suite, with no per-test selection,
        # so NoCoverage means the run was not configured the way this gate
        # assumes and the counts are lies.
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "NoCoverage")))
        self.assertEqual(code, 1)
        self.assertIn("misconfigured", msg)

    def test_an_unknown_status_fails_rather_than_being_assumed_dead(self):
        # A future Stryker status must not default into "killed".
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Pending")))
        self.assertEqual(code, 1)
        self.assertIn("Refusing to guess", msg)



class Matching(unittest.TestCase):
    """An entry claims a survivor by the code it starts on, never by its line
    (SPEC-020). An edit that only moves the code must not force a re-judgment;
    an edit to the code itself must."""

    # VTT-298
    def test_a_survivor_whose_line_moved_is_still_claimed(self):
        eq = entries(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"))
        code, msg = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= b")), eq)
        self.assertEqual(code, 0, msg)
        shifted = {n + 1: v for n, v in TS_LINES.items()}
        code, msg = run(report(("client/src/a.ts", 11, 5, "EqualityOperator", "Survived", "a >= b"),
                               lines=shifted), eq, lines=shifted)
        self.assertEqual(code, 0, msg)
        self.assertNotIn("ADJUDICATION MOVED", msg)

    # VTT-298
    def test_a_reindented_line_is_still_claimed(self):
        eq = entries(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"))
        indented = {**TS_LINES, 10: "    if (a > b) c();"}
        code, msg = run(report(("client/src/a.ts", 10, 9, "EqualityOperator", "Survived", "a >= b"),
                               lines=indented), eq, lines=indented)
        self.assertEqual(code, 0, msg)

    # VTT-298
    def test_a_reindented_multi_line_mutant_is_still_claimed(self):
        """A mutant covering two lines keeps its key when the block is
        re-indented: its span leaves out each line's indentation."""
        def lines_at(indent):
            return {**TS_LINES, 96: indent + "go(a,", 97: indent + "   b);"}
        flat, deep = lines_at(""), lines_at("        ")

        def rep(lines, col):
            r = report(("client/src/a.ts", 96, col, "MethodExpression", "Survived", "go(a, b)"), lines=lines)
            r["files"]["client/src/a.ts"]["mutants"][0]["location"]["end"] = {"line": 97, "column": col + 5}
            return r
        eq = entries(entry("client/src/a.ts", "96:1", "MethodExpression", "go(a, b)", lines=flat, span=8))
        code, msg = run(rep(flat, 1), eq, lines=flat)
        self.assertEqual(code, 0, msg)
        code, msg = run(rep(deep, 9), eq, lines=deep)
        self.assertEqual(code, 0, msg)

    # VTT-299
    def test_a_conditional_expression_entry_does_not_claim_other_code(self):
        """Its replacement is `false` whatever the clause, so only the code
        tells an edited clause from the one that was judged."""
        eq = entries(entry("client/src/a.ts", "20:51", "ConditionalExpression", "false"))
        edited = {**TS_LINES, 20: "const twenty = { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 8 };"}
        code, msg = run(report(("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false"),
                               lines=edited), eq, lines=edited)
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED ConditionalExpression at client/src/a.ts:20:51", msg)
        self.assertIn("g: 7", msg)  # the entry is reported, naming its code

    # VTT-299
    def test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed(self):
        code, msg = run(
            report(("client/src/a.ts", 42, 5, "BooleanLiteral", "Survived", "false")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "false", "reasoned")))
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", msg)
        self.assertIn("no longer survives", msg)
        self.assertIn("SURVIVED", msg)

    # VTT-298
    def test_a_survivor_with_another_replacement_at_the_entrys_code_is_not_claimed(self):
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived", "true")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "false", "reasoned")))
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)
        self.assertIn("SURVIVED", msg)

    def test_the_same_code_in_another_file_is_not_claimed(self):
        code, msg = run(
            report(("client/src/b.ts", 9, 5, "BooleanLiteral", "Survived", "false")),
            entries(entry("client/src/a.ts", "9:5", "BooleanLiteral", "false", "reasoned")))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED BooleanLiteral at client/src/b.ts:9:5", msg)

    # VTT-299
    def test_two_mutants_of_one_mutator_on_one_line_are_two_claims(self):
        """`send("a", "b");` carries two StringLiteral mutants with one
        replacement, and an entry for the first does not excuse the second."""
        first = entry("client/src/a.ts", "95:6", "StringLiteral", '""')
        code, msg = run(report(("client/src/a.ts", 95, 11, "StringLiteral", "Survived", '""')),
                        entries(first))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED StringLiteral at client/src/a.ts:95:11", msg)
        code, msg = run(report(("client/src/a.ts", 95, 6, "StringLiteral", "Survived", '""'),
                               ("client/src/a.ts", 95, 11, "StringLiteral", "Survived", '""')),
                        entries(first, entry("client/src/a.ts", "95:11", "StringLiteral", '""')))
        self.assertEqual(code, 0, msg)

    # VTT-299
    def test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated(self):
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned")))
        self.assertEqual(code, 1)
        unclaimed = [l for l in msg.splitlines() if "SURVIVED" in l]
        self.assertEqual(len(unclaimed), 1, msg)
        self.assertIn("client/src/a.ts:40:6", unclaimed[0])
        self.assertIn("nth=2", msg)

    # VTT-299
    def test_an_entry_on_one_copy_does_not_claim_another_copys_survivor(self):
        """The judged copy's mutant is killed and another copy's survives: that
        survivor is a mutant nobody judged, though its code is the same."""
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""'),
                   ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned")))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED StringLiteral at client/src/a.ts:40:6", msg)
        self.assertIn("no longer survives", msg)

    # VTT-299
    def test_two_mutants_at_one_start_with_different_extents_are_two_claims(self):
        """A ConditionalExpression on a whole test and one on its left operand
        start at one column with one replacement; only the extent tells them
        apart."""
        whole = entry("client/src/a.ts", "20:51", "ConditionalExpression", "false", span=9)
        code, msg = run(report(("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false", 3)),
                        entries(whole))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED ConditionalExpression at client/src/a.ts:20:51", msg)
        self.assertIn("span=3", msg)
        both = report(("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false", 3),
                      ("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false", 9))
        code, msg = run(both, entries(whole, entry("client/src/a.ts", "20:51", "ConditionalExpression",
                                                   "false", span=3)))
        self.assertEqual(code, 0, msg)

    def test_two_entries_on_two_identical_lines_claim_both(self):
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "first"),
                    entry("client/src/a.ts", "40:6", "StringLiteral", '""', "second")))
        self.assertEqual(code, 0, msg)

    # VTT-301
    def test_two_entries_for_one_code_and_one_survivor_leave_one_stale(self):
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "first"),
                    entry("client/src/a.ts", "40:6", "StringLiteral", '""', "second")))
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)
        self.assertNotIn("SURVIVED", msg)

    # VTT-299
    def test_a_string_literal_entry_claims_only_its_own_code(self):
        """`""` is the replacement for every string literal in a file, so only
        the code tells `label("other")` from `send("tag")`."""
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 90, 7, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "the send one")))
        self.assertEqual(code, 1)
        unclaimed = [l for l in msg.splitlines() if "SURVIVED" in l]
        self.assertEqual(len(unclaimed), 1, msg)
        self.assertIn("client/src/a.ts:90:7", unclaimed[0])

    # VTT-300
    def test_one_entry_claims_one_of_two_survivors_on_its_key(self):
        """Should two mutants ever share a key, one entry excuses one of them."""
        e = ctm.Entry("client/src/a.ts", "StringLiteral", 6, 1, 1, 4, '""', 'send("tag");', "reason")
        key = ctm.entry_key(e)
        claimed, unadjudicated, stale = ctm.claims([e], [(key, "first"), (key, "second")], [])
        self.assertEqual((claimed, unadjudicated, stale), (["first"], [(key, "second")], []))

    def test_claims_takes_survivors_before_timeouts(self):
        """A timeout is not an evaluation, so an entry is not spent on one while
        a survivor on its key is unclaimed."""
        e = ctm.Entry("client/src/a.ts", "StringLiteral", 6, 1, 1, 4, '""', 'send("tag");', "reason")
        key = ctm.entry_key(e)
        claimed, unadjudicated, stale = ctm.claims([e], [(key, "lived")], [(key, "hung")])
        self.assertEqual((claimed, unadjudicated, stale), (["lived"], [], []))

    # VTT-303
    def test_a_survivor_is_resolved_against_the_reports_own_source(self):
        """The tree has moved since Stryker ran: a line went in at the top. The
        survivor's position names the source Stryker mutated, which the report
        carries."""
        eq = entries(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"))
        shifted = {n + 1: v for n, v in TS_LINES.items()}
        code, msg = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= b")),
                        eq, lines=shifted)
        self.assertEqual(code, 0, msg)

    # VTT-304
    def test_a_report_file_without_its_source_fails_the_gate(self):
        code, msg = run(report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived"), sources={}))
        self.assertEqual(code, 1)
        self.assertIn("cannot be read", msg)

    # VTT-304
    def test_a_survivor_without_an_end_fails_the_gate(self):
        rep = report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived"))
        del rep["files"]["client/src/a.ts"]["mutants"][0]["location"]["end"]
        code, msg = run(rep)
        self.assertEqual(code, 1)
        self.assertIn("cannot be read", msg)

    # VTT-304
    def test_a_survivor_whose_line_is_not_in_the_source_fails_the_gate(self):
        code, msg = run(report(("client/src/a.ts", 9999, 5, "BooleanLiteral", "Survived")))
        self.assertEqual(code, 1)
        self.assertIn("cannot be read", msg)

    # VTT-304
    def test_a_survivor_outside_its_trimmed_line_fails_the_gate(self):
        """A column in the indentation or past the end names no code on the line."""
        indented = {**TS_LINES, 10: "    if (a > b) c();"}
        for line, col in ((10, 2), (9, 99)):
            with self.subTest(line=line, col=col):
                code, msg = run(report(("client/src/a.ts", line, col, "BooleanLiteral", "Survived"),
                                       lines=indented), lines=indented)
                self.assertEqual(code, 1)
                self.assertIn("cannot be read", msg)

    def test_an_unadjudicated_survivor_is_printed_as_a_ready_entry(self):
        """Pasted as printed, the entry parses and claims the survivor once its
        reason is written."""
        rep = report(("client/src/a.ts", 30, 6, "StringLiteral", "Survived", '""'))
        code, msg = run(rep)
        self.assertEqual(code, 1)
        block = msg.split("stated observable:\n", 1)[1].split("\n")[:3]
        self.assertEqual(block[:2], ['client/src/a.ts  StringLiteral  offset=6  span=1  nth=1  copies=4  ""',
                                     '    code: send("tag");'])
        code, msg = run(rep, entries("\n".join(block[:2]) + "\n    reasoned\n"))
        self.assertEqual(code, 0, msg)


class EntryCheck(unittest.TestCase):
    """The static check of each entry against the tree, before check() goes
    through the report's mutants (SPEC-020).

    An entry on a comment, or on code its mutator cannot rewrite, names no
    mutant Stryker makes: it could only ever go stale. Every rule here has a
    DELIBERATELY BROKEN entry that must fail, not only a sound one that passes.
    """

    def faults(self, *texts, lines=TS_LINES):
        """entry_faults over a hermetic tree, as the list of fault sentences."""
        with tempfile.TemporaryDirectory() as root:
            source_tree(root, lines)
            return [fault for _, fault in ctm.entry_faults(entries(*texts), root=root)]

    def test_a_column_is_counted_the_way_stryker_counts_it(self):
        """Stryker's columns come from Babel, which indexes JS strings in UTF-16
        code units, NOT bytes: `—` is one unit and THREE UTF-8 bytes, so a
        byte-indexed lookup lands two early. Line 22 puts one before the
        operator and line 21 one after; line 23's emoji is TWO units and one
        Python character, which separates UTF-16 from character counting."""
        self.assertEqual(self.faults(entry("client/src/a.ts", "22:23", "EqualityOperator", "a >= b")),
                         [], "offset 23 is where Stryker says the expression starts")
        self.assertEqual(self.faults(entry("client/src/a.ts", "21:5", "EqualityOperator", "a >= b")),
                         [], "an em dash AFTER the operator must not matter either")
        self.assertNotEqual(self.faults(entry("client/src/a.ts", "22:25", "EqualityOperator", "a >= b")),
                            [], "byte offset 25 is NOT the expression start and must be refused")
        self.assertEqual(self.faults(entry("client/src/a.ts", "23:21", "EqualityOperator", "a >= b")),
                         [], "an emoji is TWO UTF-16 units; offset 21 is Babel's answer")

    def test_a_key_on_a_line_comment_is_rejected(self):
        [fault] = self.faults(entry("client/src/a.ts", "70:1", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", fault)

    def test_a_key_on_a_doc_comment_is_rejected(self):
        """`/**`, which a check of `//` alone lets through."""
        [fault] = self.faults(entry("client/src/a.ts", "71:1", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", fault)

    def test_a_key_on_a_doc_comment_continuation_is_rejected(self):
        [fault] = self.faults(entry("client/src/a.ts", "72:3", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", fault)

    def test_a_mutator_with_no_position_rule_is_still_checked_for_comments(self):
        """BOTH HALVES: a NO_POSITION_RULE entry on code passes, and on a
        comment it still fails."""
        self.assertEqual(
            self.faults(entry("client/src/a.ts", "20:51", "ConditionalExpression", "false")), [],
            "a NO_POSITION_RULE mutator on real code must not be rejected outright")
        [fault] = self.faults(entry("client/src/a.ts", "70:1", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", fault, "...and the comment rule must still apply to it")

    def test_a_string_key_whose_column_does_not_open_a_string_is_rejected(self):
        [fault] = self.faults(entry("client/src/a.ts", "80:16", "StringLiteral", '"Stryker was here!"'))
        self.assertIn("opens no string", fault)

    def test_a_string_key_on_a_quote_is_accepted(self):
        self.assertEqual(self.faults(entry("client/src/a.ts", "30:6", "StringLiteral", '""')), [])

    def test_an_operator_key_is_checked_against_its_own_replacement_text(self):
        """THE OFFSET IS NOT THE OPERATOR: Stryker reports the expression's
        START, and the replacement says where the operator is, which pins the
        operands too."""
        self.assertEqual(self.faults(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b")), [])
        # Line 14 is `if (x > y) z();` — same shape, different operands.
        [fault] = self.faults(entry("client/src/a.ts", "14:5", "EqualityOperator", "a >= b"))
        self.assertIn("does not begin the expression", fault)

    def test_a_key_on_the_sibling_comparison_is_rejected(self):
        """Line 15 against line 10: same left operand, same operator, one
        operand apart. What FOLLOWS the operator is compared too."""
        [fault] = self.faults(entry("client/src/a.ts", "15:5", "EqualityOperator", "a >= b"))
        self.assertIn("does not begin the expression", fault)

    def test_an_operator_key_one_column_off_is_rejected(self):
        self.assertEqual(len(self.faults(entry("client/src/a.ts", "10:4", "EqualityOperator", "a >= b"))), 1)

    # VTT-302
    def test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported(self):
        [fault] = self.faults("client/src/a.ts  StringLiteral  offset=6  span=6  nth=1  copies=1  \"\"\n"
                              "    code: send(\"gone\");\n    reason\n")
        self.assertIn("stands 0 time(s)", fault)

    def test_a_key_naming_a_file_that_is_not_there_is_rejected(self):
        text = entry("client/src/a.ts", "9:5", "BooleanLiteral", "false").replace("a.ts", "gone.ts", 1)
        [fault] = self.faults(text)
        self.assertIn("gone.ts", fault)

    # VTT-302
    def test_an_entry_whose_copies_differ_from_the_tree_is_reported(self):
        text = entry("client/src/a.ts", "30:6", "StringLiteral", '""').replace("copies=4", "copies=3")
        [fault] = self.faults(text)
        self.assertIn("stands 4 time(s)", fault)
        self.assertIn("copies=3", fault)

    def test_two_entries_for_one_copy_are_reported(self):
        """One mutant can be excused once; a second entry on its key is a
        duplicate, not a second claim."""
        one = entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b")
        faults = self.faults(one, one)
        self.assertTrue(faults)
        self.assertTrue(all("2 entries" in f for f in faults), faults)

    def test_an_entry_whose_nth_exceeds_its_copies_is_reported(self):
        text = entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b").replace("nth=1", "nth=2")
        [fault] = self.faults(text)
        self.assertIn("nth=2", fault)

    def test_an_unknown_mutator_is_fatal_rather_than_silently_skipped(self):
        """A mutator this check cannot place must FAIL rather than pass."""
        [fault] = self.faults(entry("client/src/a.ts", "9:5", "NoSuchMutator", "x"))
        self.assertIn("NoSuchMutator", fault)

    def test_the_gate_itself_fails_and_names_the_key(self):
        """Through check(), not just the helper: a check nothing calls does not run."""
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")),
                        entries(entry("client/src/a.ts", "70:1", "ConditionalExpression", "false", "reasoned")))
        self.assertEqual(code, 1)
        self.assertIn("// a line comment", msg)
        self.assertIn("COMMENT", msg)

    def test_a_suspect_key_is_not_also_told_to_delete_itself(self):
        """An entry on a comment is stale as well as faulty; the fault is the
        one message it gets, since "remove the entry" beside it would lose the
        reasoning."""
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")),
                        entries(entry("client/src/a.ts", "70:1", "ConditionalExpression", "false", "reasoned")))
        self.assertEqual(code, 1, "a suspect key must still FAIL the gate")
        self.assertIn("// a line comment", msg)
        self.assertNotIn("remove the entry", msg.lower())

    # VTT-302
    def test_every_real_adjudication_still_points_at_its_mutant(self):
        """THE ACCEPTANCE ASSERTION, against the real tree and the real file."""
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        real = ctm.read_equivalents(os.path.join(repo, "tools", "ts-mutation-equivalents.txt"))
        self.assertGreater(len(real), 60, "the real file must not have gone empty under this")
        self.assertEqual(ctm.entry_faults(real, root=repo), [],
                         "every entry in tools/ts-mutation-equivalents.txt must name code that "
                         "stands in its file as often as it records, where its own replacement "
                         "text says")



class TimeoutAdvice(unittest.TestCase):
    """See check_mutation_test.py's TimeoutAdviceTest — same defect, same fix.

    This is the checker that printed the misaimed line 322 times on 2026-08-28,
    and client/src/view/scene-plan.ts, the file it printed 179 of them about, is
    the one a standalone container run then measured at 172 killed / 6 timed out
    / 1 survived in 298s.
    """

    def test_the_advice_names_starvation_as_well_as_a_hang(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Timeout")))
        self.assertEqual(code, 0, msg)
        # Phrases, not bare words: "blocks" also occurs in the starvation
        # clause, so asserting it alone would pass with the hang clause gone.
        self.assertIn("a test that blocks", msg.lower())
        self.assertIn("starved", msg.lower())

    def test_the_advice_gives_the_load_evidence_rather_than_a_verdict(self):
        """One run's timeout count is not evidence about a test, and the four
        recorded runs over byte-identical inputs are what shows it."""
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Timeout")))
        self.assertEqual(code, 0, msg)
        self.assertIn("re-run on an idle machine", msg.lower())

    def test_the_advice_is_printed_once_however_many_mutants_timed_out(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 3, 1, "Eq", "Timeout"),
                               ("client/src/b.ts", 4, 1, "Eq", "Timeout")))
        self.assertEqual(code, 0, msg)
        self.assertEqual(msg.lower().count("starved"), 1)
        self.assertEqual(msg.count("timed out:"), 2,
                         "every unevaluated mutant is still named, one line each")

    def test_a_run_with_no_timeouts_gives_no_advice(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")))
        self.assertEqual(code, 0, msg)
        self.assertNotIn("starved", msg.lower())


class StaleLine(unittest.TestCase):
    """What the stale line tells its reader (VTT-308). The gate does not look
    up what became of the mutant under an entry's key, and a survivor beside
    the entry may be nobody's, so the line names where to look and picks no
    remedy itself. SENTENCES is the line after "no longer survives", whole."""

    SENTENCES = (
        "under that key (the fields and code just named): this run reports no surviving "
        "or timed-out mutant under it, and that alone does not say the mutant was "
        "killed.",
        "This run found this entry's code in that file on as many lines as the entry "
        "records, so a survivor printed on other code is on another line than that "
        "code, unless the file changed between that count and the measuring of its "
        "mutants.",
        "Look above, in this same run's output, for a survivor in that file.",
        "If one is printed unadjudicated, with an entry ready to paste under it, "
        "compare its header and code line with this entry's: it may be this entry's "
        "mutant under another key.",
        "Read this entry's reason against that mutant where it stands in the file, not "
        "against the code line printed there alone.",
        "If the reason describes that mutant and holds there, re-key this entry instead "
        "of deleting it: give it the header and code line printed there.",
        "If it describes that mutant and no longer holds, this entry is wrong: remove "
        "it.",
        "If it does not describe that mutant, the survivor is another mutant.",
        "In both of those cases that survivor needs a test or an entry of its own.",
        "If one is printed as a survivor the gate cannot read (VTT-304), it may be this "
        "entry's mutant too: leave this entry as it is until a run can read that "
        "survivor.",
        "When no survivor above is this entry's mutant, make the mutation by hand and "
        "run the tests, since this run's output does not show a kill: if a test now "
        "kills the mutant, remove the entry so it cannot pre-approve a future survivor; "
        "if none does, leave the entry and find out why this run did not report its "
        "mutant.",
    )

    def split(self, msg):
        """(the stale lines, every other line of the output joined)."""
        lines = msg.splitlines()
        return ([l for l in lines if "no longer survives" in l],
                "\n".join(l for l in lines if "no longer survives" not in l))

    # VTT-308
    def test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file(self):
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""'),
                   ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned")))
        self.assertEqual(code, 1)
        [stale], rest = self.split(msg)
        self.assertIn("SURVIVED StringLiteral at client/src/a.ts:40:6", rest)
        self.assertLess(msg.index("client/src/a.ts:40:6"), msg.index(stale),
                        "the survivor is printed above")
        self.assertIn('client/src/a.ts StringLiteral (-> "") offset=6 span=1 nth=1 on `send("tag");`', stale)
        for sentence in self.SENTENCES:
            with self.subTest(sentence[:48]):
                self.assertIn(sentence, stale)
                self.assertNotIn(sentence, rest)
        self.assertTrue(stale.endswith("no longer survives " + " ".join(self.SENTENCES)), stale)
        self.assertNotIn("If the code was edited", stale)
        self.assertLess(stale.index("in this same run's output"), stale.index("remove the entry"),
                        "where to look comes before the removal")
        self.assertEqual(stale.count("no longer survives"), 1)
        self.assertNotIn("SURVIVED", stale)

    def test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it(self):
        """The gate picks no message from what it holds: beside an unadjudicated
        survivor, beside one it cannot read, or alone, the entry gets one line."""
        eq = entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned"))
        killed = ("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""')
        _, beside = run(report(killed, ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')), eq)
        _, unread = run(report(killed, ("client/src/a.ts", 9999, 6, "StringLiteral", "Survived", '""')), eq)
        _, alone = run(report(killed), eq)
        [with_survivor], _ = self.split(beside)
        [with_unreadable], others = self.split(unread)
        [without], rest = self.split(alone)
        self.assertEqual(with_survivor, without)
        self.assertEqual(with_unreadable, without)
        self.assertIn("cannot be read", others)
        self.assertLess(unread.index("cannot be read"), unread.index(with_unreadable),
                        "the survivor the gate cannot read is printed above")
        self.assertEqual(rest, "")

    def test_a_refused_entry_and_a_claimed_one_get_no_stale_line(self):
        """The other direction. An entry whose own code was edited away is
        refused and told to correct itself; an entry that claims its survivor
        is told nothing."""
        eq = entries(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"))
        edited = {**TS_LINES, 10: "if (a > q2) c();"}
        code, msg = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= q2"),
                               lines=edited), eq, lines=edited)
        self.assertEqual(code, 1)
        self.assertIn("stands 0 time(s)", msg)
        code_ok, msg_ok = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= b")), eq)
        self.assertEqual(code_ok, 0, msg_ok)
        for text in (msg, msg_ok):
            for phrase in ("no longer survives", "remove the entry") + self.SENTENCES:
                with self.subTest(phrase[:48]):
                    self.assertNotIn(phrase, text)


if __name__ == "__main__":
    unittest.main(verbosity=2)
