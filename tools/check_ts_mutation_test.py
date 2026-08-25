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


def report(*mutants):
    """mutants: (file, line, col, mutator, status[, replacement])."""
    files = {}
    for m in mutants:
        path, line, col, mutator, status = m[:5]
        replacement = m[5] if len(m) > 5 else "REP"
        files.setdefault(path, {"mutants": []})["mutants"].append({
            "mutatorName": mutator,
            "status": status,
            "replacement": replacement,
            "location": {"start": {"line": line, "column": col}},
        })
    return {"files": files}


# The source every fixture key below is keyed against.
#
# It has to be REAL source at REAL columns, because the gate now reads the tree
# at each key's line:col and rejects a key that no longer points at a mutant of
# the kind it names. Blank lines would fail every test here with a position
# complaint instead of the thing it is about.
#
# DELIBERATELY DISTINCT where two keys must stay different claims (10 vs 14),
# and DELIBERATELY IDENTICAL where a pairing must be possible (30/40/50/60,
# 90/91). That is the content anchor in a fixture: two keys pair when the
# source says they are the same statement, never merely because they share a
# file, a mutator and a replacement.
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
    40: 'send("tag");',             # three more copies of ONE statement, so a
    50: 'send("tag");',             # move between them is indistinguishable by
    60: 'send("tag");',             # content and pairable by it
    42: "const fortytwo = true;",
    70: "// a line comment",
    71: "/** a doc comment */",
    72: " * a doc-comment continuation line",
    77: "const seventyseven = true;",
    80: "const eighty = notAString;",   # col 16 is an identifier, not a quote
    90: 'label("other");',          # col 7 opens a string literal, on a
    91: 'label("other");',          # statement the `send` lines cannot be
}


def source_tree(root):
    """client/src/a.ts and b.ts, padded so the fixture line numbers are real."""
    src = os.path.join(root, "client", "src")
    os.makedirs(src, exist_ok=True)
    body = []
    while len(body) < max(TS_LINES):
        body.append(TS_LINES.get(len(body) + 1, f"const l{len(body) + 1} = 0;"))
    for name in ("a.ts", "b.ts"):
        with open(os.path.join(src, name), "w") as fh:
            fh.write("\n".join(body) + "\n")
    return root


def run(rep, equivalents=None):
    out, err = io.StringIO(), io.StringIO()
    with tempfile.TemporaryDirectory() as root:
        source_tree(root)
        code = ctm.check(rep, equivalents or {}, out=out, err=err, root=root)
    return code, out.getvalue() + err.getvalue()


def equivalents_file(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class Equivalents(unittest.TestCase):
    def test_an_entry_and_its_reason_parse(self):
        got = ctm.read_equivalents(equivalents_file(
            "client/src/a.ts 10:5 EqualityOperator a >= b\n    same observable\n"))
        self.assertEqual(got, {("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"): "same observable"})

    def test_an_entry_without_a_reason_is_an_error(self):
        # An equivalence claim forecloses ever writing a test for that mutant.
        # Two of four such claims on the Go side were WRONG; a bare entry is
        # how that happens without anyone noticing.
        with self.assertRaises(ctm.EquivalentsError):
            ctm.read_equivalents(equivalents_file("client/src/a.ts 10:5 EqualityOperator\n"))

    def test_a_reason_with_no_entry_is_an_error(self):
        with self.assertRaises(ctm.EquivalentsError):
            ctm.read_equivalents(equivalents_file("    orphaned reason\n"))

    def test_a_malformed_entry_is_an_error(self):
        with self.assertRaises(ctm.EquivalentsError):
            ctm.read_equivalents(equivalents_file("client/src/a.ts 10:5\n    reason\n"))

    def test_a_duplicate_entry_is_an_error(self):
        with self.assertRaises(ctm.EquivalentsError):
            ctm.read_equivalents(equivalents_file(
                "client/src/a.ts 10:5 Eq x\n    one\nclient/src/a.ts 10:5 Eq x\n    two\n"))


class Gate(unittest.TestCase):
    def test_all_killed_passes(self):
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")))
        self.assertEqual(code, 0, msg)

    def test_one_survivor_fails(self):
        code, msg = run(report(("client/src/a.ts", 7, 3, "Eq", "Killed"),
                               ("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived")))
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED BooleanLiteral at client/src/a.ts:9:5", msg)

    def test_an_adjudicated_survivor_passes(self):
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Survived")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "REP"): "no observable difference"})
        self.assertEqual(code, 0, msg)

    def test_an_adjudication_for_a_mutant_that_no_longer_survives_fails(self):
        # Otherwise the entry silently pre-approves whatever mutant later
        # lands at that line:col — including a real one after an edit shifts
        # the file.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Killed")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "REP"): "stale"})
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)

    def test_an_adjudication_is_keyed_by_mutator_not_just_location(self):
        # Two mutators at one location are two different claims; excusing one
        # must not excuse the other.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "EqualityOperator", "Survived")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "REP"): "different mutator"})
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED EqualityOperator", msg)

    def test_two_mutants_at_one_location_are_two_separate_claims(self):
        # Stryker emits `-> true` AND `-> false` for one ConditionalExpression.
        # Before the replacement was part of the key these collapsed into one
        # entry, so adjudicating the harmless one excused the other. Found by
        # the duplicate check on client/src/player.ts:20:51.
        rep = report(("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "true"),
                     ("client/src/a.ts", 20, 51, "ConditionalExpression", "Survived", "false"))
        code, msg = run(rep, {("client/src/a.ts", "20:51", "ConditionalExpression", "true"): "reached only when true"})
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
        # Every mutant runs the whole suite, so NoCoverage means the run was
        # not configured the way this gate assumes and the counts are lies.
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed"),
                               ("client/src/a.ts", 2, 1, "Eq", "NoCoverage")))
        self.assertEqual(code, 1)
        self.assertIn("misconfigured", msg)

    def test_an_unknown_status_fails_rather_than_being_assumed_dead(self):
        # A future Stryker status must not default into "killed".
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Pending")))
        self.assertEqual(code, 1)
        self.assertIn("Refusing to guess", msg)



class MovedAdjudication(unittest.TestCase):
    """An edit above an adjudicated mutant shifts its line, and the gate used to
    report that as TWO unrelated failures: a stale entry, and an unadjudicated
    survivor somewhere else. Nothing said they were the same mutant.

    That framing is actively dangerous. The obvious response to "this entry no
    longer survives — remove it" is to delete it, which throws away a reasoned
    adjudication AND leaves the survivor unexplained. Measured: this happened
    four separate times in one day's work, across both the Go and TS gates.
    """

    def test_a_moved_adjudication_is_reported_as_a_move_with_its_new_key(self):
        code, msg = run(
            report(("client/src/a.ts", 42, 5, "BooleanLiteral", "Survived", "false")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "false"): "reasoned"})
        self.assertEqual(code, 1)
        self.assertIn("ADJUDICATION MOVED", msg)
        self.assertIn("9:5", msg)   # where it was
        self.assertIn("42:5", msg)  # where it is now
        # And NOT as the two separate failures it used to be, because that is
        # what makes deleting a real adjudication look like the fix.
        self.assertNotIn("no longer survives", msg)
        self.assertNotIn("SURVIVED", msg)

    def test_a_move_still_fails_the_gate(self):
        # It is a re-key, not a pass. An entry pointing at the wrong line would
        # pre-approve whatever later lands there.
        code, _ = run(
            report(("client/src/a.ts", 42, 5, "BooleanLiteral", "Survived", "false")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "false"): "reasoned"})
        self.assertEqual(code, 1)

    def test_a_different_replacement_is_not_a_move(self):
        # Same mutator and file, DIFFERENT mutation: two distinct claims, and
        # pairing them would re-key an adjudication onto a mutant nobody judged.
        code, msg = run(
            report(("client/src/a.ts", 42, 5, "BooleanLiteral", "Survived", "true")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "false"): "reasoned"})
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)
        self.assertIn("SURVIVED", msg)

    def test_a_move_across_files_is_not_a_move(self):
        # Same mutator and replacement in a different file is a different
        # mutant. Pairing across files would silently excuse it.
        code, msg = run(
            report(("client/src/b.ts", 9, 5, "BooleanLiteral", "Survived", "false")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "false"): "reasoned"})
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", msg)

    def test_two_candidates_at_once_are_not_paired(self):
        # Ambiguous: two survivors match one stale entry equally well. Guessing
        # which is the move would re-key onto a mutant nobody judged, so report
        # them the old way and let a human decide.
        code, msg = run(
            report(("client/src/a.ts", 42, 5, "BooleanLiteral", "Survived", "false"),
                   ("client/src/a.ts", 77, 5, "BooleanLiteral", "Survived", "false")),
            {("client/src/a.ts", "9:5", "BooleanLiteral", "false"): "reasoned"})
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", msg)
        self.assertIn("no longer survives", msg)


class KeyPosition(unittest.TestCase):
    """A key must still point at a mutant of the kind it names.

    THIS IS THE DANGEROUS CLASS OF DRIFT. A key that merely MOVED fails the
    gate anyway — a stale entry beside an unadjudicated survivor. A key that
    landed on a COMMENT fails nothing: Stryker generates no mutant there, so it
    never appears in the report, and the entry sits in the file pre-approving
    whatever lands on that line next. `wire.ts 292:13` and `316:7` sat on
    comment lines for three days; a canvas.ts key landed on one within an hour
    of being written, because a comment edit above it shifted the line.

    The trap these tests are written against is recorded in the equivalents
    file's own header: the hand-rolled version of this check tested
    `startswith("//")`, which a `/**` doc comment does not match, and had no
    branch for the mutator on the entry that had drifted — so it reported "0
    suspect" TWICE over a genuinely broken key. Every rule below therefore has
    a DELIBERATELY BROKEN key that must fail, not only a good key that passes.
    """

    def faults(self, *keys):
        """suspect_positions over a hermetic tree, as {"line:col": complaint}."""
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            entries = {k: "reason" for k in keys}
            return {k[1]: fault for k, fault in ctm.suspect_positions(entries, root=root)}

    def test_a_column_is_counted_the_way_stryker_counts_it(self):
        """Stryker's columns come from Babel, which indexes JS strings — UTF-16
        code units, NOT bytes.

        source_line's docstring asserted the opposite ("the column is a byte
        offset"), so any line with a non-ASCII character before the mutant was
        read at the wrong place: `—` is one character and THREE UTF-8 bytes, so
        a byte-indexed lookup lands two early and the operator is not where the
        checker looks.

        The direction is the safe one — a sound key is REJECTED rather than a
        broken one accepted — but it is still a gate refusing correct work, and
        this repo's comments are full of em dashes. Line 22 of the fixture puts
        one before the operator; line 21 puts one after, where byte and
        character columns agree, so the two together separate "handles
        non-ASCII" from "happens to work when it appears late"."""
        self.assertEqual(self.faults(("client/src/a.ts", "22:23", "EqualityOperator", "a >= b")),
                         {}, "column 23 is where Stryker says the expression starts")
        self.assertEqual(self.faults(("client/src/a.ts", "21:5", "EqualityOperator", "a >= b")),
                         {}, "an em dash AFTER the operator must not matter either")
        # ...and the check still bites on this line: a column that is wrong in
        # Stryker's own counting is still refused, so the fix widened nothing.
        self.assertNotEqual(self.faults(("client/src/a.ts", "22:25", "EqualityOperator", "a >= b")),
                            {}, "byte column 25 is NOT the expression start and must be refused")
        # THE ASTRAL CASE, and without it this test does not pin its own name.
        # Every character above is BMP, where UTF-16 units and Python characters
        # agree — so a character-counting implementation passes all three
        # assertions and the docstring's design decision goes unchecked. Found
        # by review, fault-injected to confirm: swapping byte_offset for a
        # character version leaves those three green and fails only this one.
        self.assertEqual(self.faults(("client/src/a.ts", "23:21", "EqualityOperator", "a >= b")),
                         {}, "an emoji is TWO UTF-16 units; column 21 is Babel's answer")

    def test_a_key_on_a_line_comment_is_rejected(self):
        faults = self.faults(("client/src/a.ts", "70:1", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", faults.get("70:1", ""))

    def test_a_key_on_a_doc_comment_is_rejected(self):
        """`/**`, the exact opener the shipped `startswith("//")` let through."""
        faults = self.faults(("client/src/a.ts", "71:1", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", faults.get("71:1", ""))

    def test_a_key_on_a_doc_comment_continuation_is_rejected(self):
        faults = self.faults(("client/src/a.ts", "72:3", "ConditionalExpression", "false"))
        self.assertIn("COMMENT", faults.get("72:3", ""))

    def test_a_mutator_with_no_position_rule_is_still_checked_for_comments(self):
        """ConditionalExpression spans a whole clause, so its column says
        nothing about a token — but "not on a comment" still applies, and it is
        the rule that catches the drift that actually happened."""
        self.assertEqual(self.faults(("client/src/a.ts", "20:51", "ConditionalExpression", "false")), {})

    def test_a_string_key_whose_column_does_not_open_a_string_is_rejected(self):
        faults = self.faults(("client/src/a.ts", "80:16", "StringLiteral", '"Stryker was here!"'))
        self.assertIn("80:16", faults)

    def test_a_string_key_on_a_quote_is_accepted(self):
        self.assertEqual(self.faults(("client/src/a.ts", "30:6", "StringLiteral", '""')), {})

    def test_an_operator_key_is_checked_against_its_own_replacement_text(self):
        """THE COLUMN IS NOT THE OPERATOR. Stryker reports a binary expression's
        START, so `undo.ts 39:26` points at the `e` of `e.sequence > best`, not
        at the `>`. The replacement — `e.sequence >= best` — is what says where
        the operator is, and comparing the two pins the OPERANDS as well as the
        operator: a key that drifted onto another comparison is rejected even
        though a `>` sits in the same place.
        """
        good = ("client/src/a.ts", "10:5", "EqualityOperator", "a >= b")
        self.assertEqual(self.faults(good), {})
        # Line 14 is `if (x > y) z();` — same shape, different operands.
        drifted = ("client/src/a.ts", "14:5", "EqualityOperator", "a >= b")
        self.assertIn("14:5", self.faults(drifted))

    def test_a_key_on_the_sibling_comparison_is_rejected(self):
        """THE WORST OF THE SIX KEYS the 2026-08-21 sweep had to fix, in this
        file's own words: it "landed on the SIBLING comparison, right shape
        wrong operand — the worst of the six, because it looks plausible at the
        new location". `env.sequence > this.seenSeq` one line from
        `env.sequence > this.lastSeq`.

        Line 15 is that shape against line 10: same left operand, same
        operator, one operand apart. Checking only what precedes the operator
        accepts it, which is why what FOLLOWS the operator is compared too.
        """
        self.assertIn("15:5", self.faults(("client/src/a.ts", "15:5", "EqualityOperator", "a >= b")))

    def test_an_operator_key_one_column_off_is_rejected(self):
        self.assertIn("10:4", self.faults(("client/src/a.ts", "10:4", "EqualityOperator", "a >= b")))

    def test_a_key_naming_a_line_that_is_not_there_is_rejected(self):
        self.assertIn("9999:5", self.faults(("client/src/a.ts", "9999:5", "StringLiteral", '""')))

    def test_a_key_naming_a_file_that_is_not_there_is_rejected(self):
        self.assertIn("9:5", self.faults(("client/src/gone.ts", "9:5", "StringLiteral", '""')))

    def test_an_unknown_mutator_is_fatal_rather_than_silently_skipped(self):
        """THE SECOND HALF OF THE RECORDED BUG: the shipped check had no branch
        for one entry's mutator and skipped it without a word, which is how it
        said "0 suspect" over a key that had drifted. A mutator this gate
        cannot place must FAIL rather than pass — the alternative is the defect.
        """
        faults = self.faults(("client/src/a.ts", "9:5", "NoSuchMutator", "x"))
        self.assertIn("NoSuchMutator", faults.get("9:5", ""))

    def test_the_gate_itself_fails_and_names_the_key(self):
        """Through check(), not just the helper: a check nothing calls is a
        check that does not run."""
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")),
                        {("client/src/a.ts", "70:1", "ConditionalExpression", "false"): "reasoned"})
        self.assertEqual(code, 1)
        self.assertIn("70:1", msg)
        self.assertIn("COMMENT", msg)

    def test_a_suspect_key_is_not_also_told_to_delete_itself(self):
        """TWO MESSAGES THAT CONTRADICT EACH OTHER ARE WORSE THAN ONE.

        A key on a comment has no mutant, so it is stale as well as suspect,
        and the generic stale line fires beside the position one: "re-key it,
        do not delete it" immediately under "Remove the entry". A reader
        following the second loses the reasoning, which is the exact harm this
        whole change exists to stop. The position diagnosis is strictly the
        more informative of the two, so it is the one that survives — and the
        entry is still reported, and the gate still fails.
        """
        code, msg = run(report(("client/src/a.ts", 1, 1, "Eq", "Killed")),
                        {("client/src/a.ts", "70:1", "ConditionalExpression", "false"): "reasoned"})
        self.assertEqual(code, 1, "a suspect key must still FAIL the gate")
        self.assertIn("70:1", msg)
        self.assertNotIn("Remove the entry", msg)

    def test_every_real_adjudication_still_points_at_its_mutant(self):
        """THE ACCEPTANCE ASSERTION, against the real tree and the real file.

        This is what would have caught the four comment-line keys, and it is
        also what keeps the table honest: a rule that rejected any of the 73
        recorded adjudications would be a bug in the rule, not a finding.
        """
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        entries = ctm.read_equivalents(os.path.join(repo, "tools", "ts-mutation-equivalents.txt"))
        self.assertGreater(len(entries), 60, "the real file must not have gone empty under this")
        self.assertEqual(ctm.suspect_positions(entries, root=repo), [],
                         "every key in tools/ts-mutation-equivalents.txt must still point at the "
                         "expression its own replacement text names")


class AmbiguousMove(unittest.TestCase):
    """Pairing when a signature is not unique, and never advising a deletion
    when a pairing exists.

    The gate used to refuse every ambiguous group and tell the reader to DELETE
    the entries. That is the worst of the three options: it destroys reasoning
    somebody did AND leaves a real survivor unexplained. Measured on
    2026-08-25, on app.ts's two `history.replaceState` calls — two identical
    statements whose StringLiteral replacement is `"Stryker was here!"` for
    every string literal in the file, so no content can tell them apart.
    """

    def test_two_entries_and_two_survivors_pair_by_position_order(self):
        """Nothing in the source can say which entry became which survivor, and
        it does not matter: the statements are identical, so the adjudications
        are interchangeable. Reported AS an order-based pairing, so a reader
        who knows better can object."""
        code, msg = run(
            report(("client/src/a.ts", 50, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 60, 6, "StringLiteral", "Survived", '""')),
            {("client/src/a.ts", "30:6", "StringLiteral", '""'): "first",
             ("client/src/a.ts", "40:6", "StringLiteral", '""'): "second"})
        self.assertEqual(code, 1, "a re-key is still a failure, not a pass")
        self.assertIn("POSITION ORDER", msg)
        self.assertIn("30:6 -> 50:6", msg)
        self.assertIn("40:6 -> 60:6", msg)
        self.assertNotIn("no longer survives", msg)

    def test_an_anchorless_ordered_pairing_does_not_claim_a_shared_source_line(self):
        """The justification must describe the signature that actually paired.

        pair_moves has two passes: anchored (file, mutator, replacement AND the
        source line) and coarse (the first three only). The ordered wording
        claimed all four, which is false whenever the coarse pass is what
        matched — and that is not an exotic case, it is every entry whose file
        cannot be read, including the one the test above uses. A reader
        objecting to an order-based pairing needs to know which facts were
        actually compared; telling them the source lines matched when nothing
        read a source line invites them to accept a pairing on evidence that
        was never gathered."""
        code, msg = run(
            report(("client/src/does-not-exist.ts", 50, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/does-not-exist.ts", 60, 6, "StringLiteral", "Survived", '""')),
            {("client/src/does-not-exist.ts", "30:6", "StringLiteral", '""'): "first",
             ("client/src/does-not-exist.ts", "40:6", "StringLiteral", '""'): "second"})
        self.assertEqual(code, 1)
        self.assertIn("POSITION ORDER", msg)
        self.assertNotIn("source line", msg)

    def test_mismatched_counts_still_refuse_to_guess(self):
        """Two entries, one survivor. There is no pairing, only a choice, and
        the gate does not make choices — it reports both sides and fails."""
        code, msg = run(
            report(("client/src/a.ts", 50, 6, "StringLiteral", "Survived", '""')),
            {("client/src/a.ts", "30:6", "StringLiteral", '""'): "first",
             ("client/src/a.ts", "40:6", "StringLiteral", '""'): "second"})
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", msg)
        self.assertNotIn("POSITION ORDER", msg)
        self.assertIn("no longer survives", msg)
        self.assertIn("SURVIVED", msg)

    def test_the_anchor_separates_two_statements_sharing_a_replacement(self):
        """What the anchor buys, and it is a pairing the counts alone refuse.

        Two entries and three survivors share (file, mutator, replacement), so
        the old signature gives 2-against-3 and gives up on all five. The
        trimmed source line separates them: `label("other")` has one entry and
        one survivor and pairs; the three `send("tag")` copies are 1-against-2
        and are still reported both ways.
        """
        code, msg = run(
            report(("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 50, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 91, 7, "StringLiteral", "Survived", '""')),
            {("client/src/a.ts", "30:6", "StringLiteral", '""'): "the send one",
             ("client/src/a.ts", "90:7", "StringLiteral", '""'): "the label one"})
        self.assertEqual(code, 1)
        self.assertIn("ADJUDICATION MOVED", msg)
        self.assertIn("90:7", msg)
        self.assertIn("91:7", msg)
        # The unpairable half is still reported both ways, unguessed.
        self.assertIn("30:6", msg)
        self.assertIn("no longer survives", msg)

    def test_a_pairing_is_never_advice_to_delete(self):
        """The property the whole of Part B is for, asserted on the message
        rather than inferred from the pairing."""
        _, msg = run(
            report(("client/src/a.ts", 50, 6, "StringLiteral", "Survived", '""'),
                   ("client/src/a.ts", 60, 6, "StringLiteral", "Survived", '""')),
            {("client/src/a.ts", "30:6", "StringLiteral", '""'): "first",
             ("client/src/a.ts", "40:6", "StringLiteral", '""'): "second"})
        self.assertIn("RE-KEY", msg)
        self.assertNotIn("Remove the entry", msg)


if __name__ == "__main__":
    unittest.main(verbosity=2)
