#!/usr/bin/env python3
"""Boundary tests for check-ts-coverage.py.

Same reasoning as check_coverage_test.py: this script IS the gate, so a bug in
it makes every client coverage number a lie. Its Go sibling shipped untested
and a review found two ways it could PASS when it should FAIL; those two
shapes are pinned here too, along with the one this script got wrong on its
FIRST real run — a floor written at 2dp that rounded UP above the number it
came from, failing the gate against the very run that produced it.

Tests assert on EXIT CODES and on what is reported. Run:
  python3 tools/check_ts_coverage_test.py
"""

import importlib.util
import io
import pathlib
import os
import unittest

_spec = importlib.util.spec_from_file_location(
    "check_ts_coverage",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-ts-coverage.py"))
ctc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(ctc)

# Every exemption, not just the first. The helper below has to satisfy the
# gate's staleness rule for ALL of them: an EXCLUDED entry naming a file that
# is absent from the source tree or the report is itself an error, so a fixture
# that supplied only one would fail the moment a second exemption was added —
# which is exactly what happened when client/src/main.ts joined the list.
EXCLUDED_FILES = list(ctc.EXCLUDED)
EXCLUDED_FILE = EXCLUDED_FILES[0]


def lcov(*records):
    """records: (file, [hit_counts]) -> an lcov report."""
    out = []
    for path, hits in records:
        out.append(f"TN:\nSF:{path}")
        for i, h in enumerate(hits, 1):
            out.append(f"DA:{i},{h}")
        out.append("end_of_record")
    return "\n".join(out) + "\n"


def run(lcov_text, thresholds, expected, with_excluded=True):
    """Run the gate.

    with_excluded appends EVERY EXCLUDED file to both the report and the
    expected set, because the gate requires each exemption to name a file that
    still exists and is still measured. Tests about THAT rule pass
    with_excluded=False.
    """
    if with_excluded:
        lcov_text = lcov_text + lcov(*((f, [1, 1, 0]) for f in EXCLUDED_FILES))
        expected = list(expected) + EXCLUDED_FILES
    err = io.StringIO()
    # Success output goes to a sink too: the gate's own tests must not print
    # "all files pass" lines into the gate's output, where they read as
    # results of the real run.
    code = ctc.check(lcov_text, thresholds, set(expected), err, out=io.StringIO())
    return code, err.getvalue()


class ParseLcov(unittest.TestCase):
    def test_counts_covered_and_total(self):
        got = ctc.parse_lcov(lcov(("a.ts", [1, 0, 5, 0])))
        self.assertEqual(got, {"a.ts": (2, 4)})

    def test_multiple_records_are_kept_apart(self):
        got = ctc.parse_lcov(lcov(("a.ts", [1]), ("b.ts", [0, 0])))
        self.assertEqual(got, {"a.ts": (1, 1), "b.ts": (0, 2)})

    def test_a_record_with_no_end_marker_is_still_returned(self):
        # A truncated report must not silently drop its last file, which is
        # the one most likely to be missing a floor.
        got = ctc.parse_lcov("SF:a.ts\nDA:1,1\n")
        self.assertEqual(got, {"a.ts": (1, 1)})


class Thresholds(unittest.TestCase):
    def test_comments_and_blank_lines_are_ignored(self):
        got = ctc.parse_thresholds("# note\n\nclient/src/a.ts 90.0  # trailing\n")
        self.assertEqual(got, {"client/src/a.ts": 90.0})

    def test_an_empty_file_is_an_error_not_an_empty_gate(self):
        # Otherwise deleting the contents disables the gate entirely while it
        # keeps reporting success.
        with self.assertRaises(ValueError):
            ctc.parse_thresholds("# only a comment\n")

    def test_a_malformed_line_is_an_error(self):
        with self.assertRaises(ValueError):
            ctc.parse_thresholds("client/src/a.ts\n")

    def test_a_non_numeric_floor_is_an_error(self):
        with self.assertRaises(ValueError):
            ctc.parse_thresholds("client/src/a.ts ninety\n")

    def test_a_duplicated_file_is_an_error(self):
        # Last-wins would let a weaker second entry override a stronger first
        # one with nothing to show for it in review.
        with self.assertRaises(ValueError):
            ctc.parse_thresholds("client/src/a.ts 90\nclient/src/a.ts 86\n")


class NonFiniteFloors(unittest.TestCase):
    """A review found nan disabled a file's gate outright, proven end-to-end.

    float() accepts it; then `nan < 85.0` is False so the minimum check passes
    it, and `pct + 1e-9 < nan` is False so the file can never fail. The Go
    sibling is immune only because it tests `pct >= want` — this script
    inverted the comparison, which turned a loud failure into a silent
    exemption.
    """

    def test_nan_is_rejected_at_parse(self):
        for text in ("client/src/a.ts nan\n", "client/src/a.ts NaN\n"):
            with self.subTest(text=text), self.assertRaises(ValueError):
                ctc.parse_thresholds(text)

    def test_infinities_are_rejected_at_parse(self):
        for text in ("client/src/a.ts inf\n", "client/src/a.ts -inf\n",
                     "client/src/a.ts 1e400\n"):
            with self.subTest(text=text), self.assertRaises(ValueError):
                ctc.parse_thresholds(text)

    def test_a_nan_floor_cannot_pass_the_gate(self):
        code, err = run(lcov(("client/src/a.ts", [0] * 10)),
                        "client/src/a.ts nan\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)


class StaleExemptions(unittest.TestCase):
    """EXCLUDED is the only hole in the gate, so it gets the staleness check
    the floors already had. Without it, moving the types out of state.ts and
    putting real logic at that path leaves the new file permanently ungated
    while the output still says '(1 excluded)'."""

    def test_an_exemption_for_a_deleted_file_fails(self):
        code, err = run(lcov(("client/src/a.ts", [1])),
                        "client/src/a.ts 100.0\n", ["client/src/a.ts"],
                        with_excluded=False)
        self.assertEqual(code, 1)
        self.assertIn(EXCLUDED_FILE, err)
        self.assertIn("no longer exists", err)

    def test_an_exemption_for_an_unmeasured_file_fails(self):
        # The recorded reason is about what the file MEASURES, so it has to
        # stay measured for that reason to be checkable.
        code, err = run(lcov(("client/src/a.ts", [1])),
                        "client/src/a.ts 100.0\n",
                        ["client/src/a.ts", EXCLUDED_FILE], with_excluded=False)
        self.assertEqual(code, 1)
        self.assertIn("absent from the coverage report", err)


class EmptyExpected(unittest.TestCase):
    def test_no_source_files_is_an_error_not_a_pass(self):
        # client/src moving is enough to empty this. The path errors that
        # follow are then "fixed" by rewriting the thresholds file, leaving
        # the absent-from-report check — the entire reason this gate is not a
        # bunfig coverageThreshold line — dead, and the gate green.
        code, err = run(lcov(("client/src/a.ts", [1])),
                        "client/src/a.ts 100.0\n", [], with_excluded=False)
        self.assertEqual(code, 1)
        self.assertIn("no source files found", err)


class DuplicateRecords(unittest.TestCase):
    def test_a_second_record_for_one_file_cannot_erase_the_first(self):
        # Not reachable with bun 1.3.10, which emits one record per resolved
        # path. Pinned because nothing would notice if that changed: last-wins
        # would report 100% for a file whose first record was 0/10.
        text = lcov(("client/src/a.ts", [0] * 10)) + lcov(("client/src/a.ts", [1] * 10))
        code, err = run(text, "client/src/a.ts 100.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)


class SourceWalk(unittest.TestCase):
    """expected_files() and walk() in all-modules.test.ts are two hand-synced
    implementations of one rule. These pin this side of it."""

    def test_the_extension_rule(self):
        import tempfile, os
        root = tempfile.mkdtemp()
        src = os.path.join(root, "client", "src", "view")
        os.makedirs(src)
        for name in ("a.ts", "b.tsx", "c.mts", "d.cts", "e.d.ts", "f.js", "g.json"):
            open(os.path.join(src, name), "w").close()
        got = {p.split("/")[-1] for p in ctc.expected_files(pathlib.Path(root))}
        self.assertEqual(got, {"a.ts", "b.tsx", "c.mts", "d.cts"})

    def test_a_symlinked_directory_is_not_descended_into(self):
        # walk() follows with statSync unless it lstats; rglob never does.
        # Divergence puts files in `measured` that are not in `expected`.
        import tempfile, os
        root = tempfile.mkdtemp()
        src = os.path.join(root, "client", "src")
        outside = os.path.join(root, "shared")
        os.makedirs(src); os.makedirs(outside)
        open(os.path.join(outside, "x.ts"), "w").close()
        os.symlink(outside, os.path.join(src, "linked"))
        open(os.path.join(src, "own.ts"), "w").close()
        got = {p.split("/")[-1] for p in ctc.expected_files(pathlib.Path(root))}
        self.assertEqual(got, {"own.ts"})


class Gate(unittest.TestCase):
    def test_passes_when_every_file_meets_its_floor(self):
        code, err = run(lcov(("client/src/a.ts", [1] * 9 + [0])),
                        "client/src/a.ts 90.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 0, err)

    def test_fails_when_a_file_is_below_its_floor(self):
        code, err = run(lcov(("client/src/a.ts", [1] * 8 + [0, 0])),
                        "client/src/a.ts 90.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("below its floor", err)

    def test_a_floor_equal_to_the_measurement_passes(self):
        # 37/38 is 97.368421...; a floor of 97.36 must pass and 97.37 must
        # not. This exact case failed the gate on its first real run.
        code, err = run(lcov(("client/src/a.ts", [1] * 37 + [0])),
                        "client/src/a.ts 97.36\n", ["client/src/a.ts"])
        self.assertEqual(code, 0, err)
        code, _ = run(lcov(("client/src/a.ts", [1] * 37 + [0])),
                      "client/src/a.ts 97.37\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)

    def test_a_source_file_absent_from_the_report_fails(self):
        # THE case this gate exists for: bun reports only imported files, so
        # an untested file is missing rather than 0%.
        code, err = run(lcov(("client/src/a.ts", [1])),
                        "client/src/a.ts 100.0\n",
                        ["client/src/a.ts", "client/src/ghost.ts"])
        self.assertEqual(code, 1)
        self.assertIn("ghost.ts", err)
        self.assertIn("absent from the coverage report", err)

    def test_a_measured_file_with_no_floor_fails(self):
        # Set equality: narrowing the thresholds file cannot shrink the gate
        # to whatever remains.
        code, err = run(lcov(("client/src/a.ts", [1]), ("client/src/b.ts", [0])),
                        "client/src/a.ts 100.0\n",
                        ["client/src/a.ts", "client/src/b.ts"])
        self.assertEqual(code, 1)
        self.assertIn("client/src/b.ts", err)
        self.assertIn("no recorded floor", err)

    def test_a_misspelled_threshold_key_fails(self):
        # Otherwise one typo silently drops a ratcheted floor to nothing and
        # the file it named is reported as ungated.
        code, err = run(lcov(("client/src/a.ts", [1] * 9 + [0])),
                        "client/src/a.ts 90.0\nclient/src/aa.ts 99.0\n",
                        ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("client/src/aa.ts", err)
        self.assertIn("was not measured", err)

    def test_a_floor_below_the_minimum_is_rejected(self):
        # The 85% minimum is the number the whole file cites; without this it
        # would be the one number nothing checks.
        code, err = run(lcov(("client/src/a.ts", [1] * 8 + [0, 0])),
                        "client/src/a.ts 80.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("below the 85% minimum", err)

    def test_the_ungated_prefixes_are_anchored_at_a_path_boundary(self):
        # A bare "client/test" prefix would also swallow client/testdata. The
        # tier-1 grep had exactly this bug with internal/gateway vs
        # internal/gatewayx, and every gate stayed green over it.
        code, err = run(lcov(("client/src/a.ts", [1]),
                             ("client/testdata/seed.ts", [0, 0])),
                        "client/src/a.ts 100.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("client/testdata/seed.ts", err)
        self.assertIn("no recorded floor", err)

    def test_generated_and_helper_code_is_neither_gated_nor_demanded(self):
        code, err = run(
            lcov(("client/src/a.ts", [1]),
                 ("contract/gen/ts/x_pb.ts", [0, 0]),
                 ("client/test/support/dom.ts", [0])),
            "client/src/a.ts 100.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 0, err)

    def test_an_excluded_file_is_not_required_to_have_a_floor(self):
        code, err = run(lcov(("client/src/a.ts", [1])),
                        "client/src/a.ts 100.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 0, err)

    def test_an_excluded_file_may_not_also_carry_a_floor(self):
        # Two sources of truth for one file: the exclusion silently wins and
        # the floor reads as enforced when it is not.
        code, err = run(lcov(("client/src/a.ts", [1])),
                        f"client/src/a.ts 100.0\n{EXCLUDED_FILE} 95.0\n",
                        ["client/src/a.ts"])
        self.assertEqual(code, 1, err)

    def test_a_file_with_no_executable_lines_fails_rather_than_passing_vacuously(self):
        code, err = run(lcov(("client/src/a.ts", [])),
                        "client/src/a.ts 90.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("zero executable lines", err)

    def test_an_empty_report_fails_loudly(self):
        # A run that produced nothing must not read as "all files pass".
        code, err = run("", "client/src/a.ts 90.0\n", ["client/src/a.ts"])
        self.assertEqual(code, 1)
        self.assertIn("absent from the coverage report", err)


if __name__ == "__main__":
    unittest.main(verbosity=2)
