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
    """mutants: (file, line, col, mutator, status)."""
    files = {}
    for path, line, col, mutator, status in mutants:
        files.setdefault(path, {"mutants": []})["mutants"].append({
            "mutatorName": mutator,
            "status": status,
            "location": {"start": {"line": line, "column": col}},
        })
    return {"files": files}


def run(rep, equivalents=None):
    out, err = io.StringIO(), io.StringIO()
    code = ctm.check(rep, equivalents or {}, out=out, err=err)
    return code, out.getvalue() + err.getvalue()


def equivalents_file(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class Equivalents(unittest.TestCase):
    def test_an_entry_and_its_reason_parse(self):
        got = ctm.read_equivalents(equivalents_file(
            "client/src/a.ts 10:5 EqualityOperator\n    same observable\n"))
        self.assertEqual(got, {("client/src/a.ts", "10:5", "EqualityOperator"): "same observable"})

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
                "client/src/a.ts 10:5 Eq\n    one\nclient/src/a.ts 10:5 Eq\n    two\n"))


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
            {("client/src/a.ts", "9:5", "BooleanLiteral"): "no observable difference"})
        self.assertEqual(code, 0, msg)

    def test_an_adjudication_for_a_mutant_that_no_longer_survives_fails(self):
        # Otherwise the entry silently pre-approves whatever mutant later
        # lands at that line:col — including a real one after an edit shifts
        # the file.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "BooleanLiteral", "Killed")),
            {("client/src/a.ts", "9:5", "BooleanLiteral"): "stale"})
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", msg)

    def test_an_adjudication_is_keyed_by_mutator_not_just_location(self):
        # Two mutators at one location are two different claims; excusing one
        # must not excuse the other.
        code, msg = run(
            report(("client/src/a.ts", 9, 5, "EqualityOperator", "Survived")),
            {("client/src/a.ts", "9:5", "BooleanLiteral"): "different mutator"})
        self.assertEqual(code, 1)
        self.assertIn("SURVIVED EqualityOperator", msg)

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


if __name__ == "__main__":
    unittest.main(verbosity=2)
