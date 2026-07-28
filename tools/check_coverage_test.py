#!/usr/bin/env python3
"""Boundary tests for check-coverage.py.

This script IS the coverage gate, so a bug in it makes every other package's
number a lie. It was shipped untested and a review immediately found two ways
it could PASS when it should FAIL (a misspelled threshold key silently
dropping a ratcheted floor; a shrunken measured set reporting success over
whatever remained). Both are pinned below.

Tests assert on EXIT CODES and on which packages are reported — the gate's
boundary — never on internal helper shapes. Run: python3 tools/check_coverage_test.py
"""

import importlib.util
import io
import os
import tempfile
import unittest

_spec = importlib.util.spec_from_file_location(
    "check_coverage", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-coverage.py"))
cc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cc)

PKG = "github.com/PatrikLager/vtt-platform/internal"


def profile(*blocks, mode="set"):
    """Write a temp profile. Each block is (pkgpath, file, nstmt, count)."""
    fh = tempfile.NamedTemporaryFile("w", suffix=".out", delete=False)
    fh.write(f"mode: {mode}\n")
    for i, (pkg, name, nstmt, count) in enumerate(blocks):
        fh.write(f"{pkg}/{name}:{i + 1}.1,{i + 1}.20 {nstmt} {count}\n")
    fh.close()
    return fh.name


def expectfile(*pkgs):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write("\n".join(pkgs) + "\n")
    fh.close()
    return fh.name


def thresholds(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class GateTest(unittest.TestCase):
    def gate(self, thresholds_path, profiles, expected_path=None):
        """profiles is a list for call-site compatibility; the gate takes one.

        Merging is `go tool covdata merge`'s job now, so passing more than one
        is a caller bug rather than a supported mode.
        """
        assert len(profiles) <= 1, "the gate takes a single merged profile"
        out, err = io.StringIO(), io.StringIO()
        path = profiles[0] if profiles else "/nonexistent/profile.out"
        code = cc.run(thresholds_path, path, expected_path, out=out, err=err)
        return code, out.getvalue(), err.getvalue()

    # --- the two ways the first version could pass when it should fail ---

    def test_misspelled_threshold_key_is_fatal(self):
        """A stale/typo'd package name must not silently degrade to the 85 default.

        Regression: 'internal/engnie' for 'internal/engine' dropped a 97.5
        floor to 85.0 and exited 0.
        """
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        t = thresholds(f"{PKG}/engnie 97.5\n")
        code, _, err = self.gate(t, [p])
        self.assertEqual(code, 1)
        self.assertIn("engnie", err)
        self.assertIn("not being enforced", err)

    def test_measured_package_without_a_floor_is_fatal(self):
        """Every measured package must have a recorded floor.

        This is what makes thresholds == measured BY CONSTRUCTION, which is in
        turn what makes the stale-key check sufficient to catch a shrinking
        measured set. Without it a package sitting at the default floor has no
        entry, so nothing notices when it stops being measured — the round-2
        review proved exactly that hole.
        """
        p = profile((f"{PKG}/engine", "apply.go", 100, 1),
                    (f"{PKG}/newpkg", "x.go", 92, 1), (f"{PKG}/newpkg", "x.go", 8, 0))
        t = thresholds(f"{PKG}/engine 85.0\n")
        code, _, err = self.gate(t, [p])
        self.assertEqual(code, 1)
        self.assertIn("newpkg", err)
        self.assertIn("no floor", err)

    def test_missing_thresholds_file_is_fatal(self):
        """Deleting or mistyping the ratchet file must not silently pass.

        Regression: read_thresholds returned {} for a missing path, so all
        thirteen floors fell to 85 and the gate exited 0 with engine's 97.7%
        measured against 85.
        """
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        code, _, err = self.gate("/nonexistent/coverage-thresholds.txt", [p])
        self.assertEqual(code, 1)
        self.assertIn("does not exist", err)

    def test_empty_thresholds_file_is_fatal(self):
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        code, _, err = self.gate(thresholds("# only a comment\n"), [p])
        self.assertEqual(code, 1)
        self.assertIn("no floors", err)

    # --- core enforcement ---

    def test_below_floor_fails_and_names_the_package(self):
        p = profile((f"{PKG}/engine", "apply.go", 80, 1), (f"{PKG}/engine", "apply.go", 20, 0))
        t = thresholds(f"{PKG}/engine 85.0\n")
        code, _, err = self.gate(t, [p])
        self.assertEqual(code, 1)
        self.assertIn("80.0%", err)
        self.assertIn("engine", err)

    def test_at_floor_exactly_passes(self):
        """85.0 against a floor of 85.0 must pass — the floor is inclusive."""
        p = profile((f"{PKG}/engine", "apply.go", 85, 1), (f"{PKG}/engine", "apply.go", 15, 0))
        t = thresholds(f"{PKG}/engine 85.0\n")
        code, _, _ = self.gate(t, [p])
        self.assertEqual(code, 0)

    def test_package_below_the_minimum_fails_even_with_a_matching_entry(self):
        """A new package at 80% fails, entry or no entry.

        Renamed from test_package_with_no_threshold_entry_uses_the_default_floor,
        which stopped describing its own body once a missing entry became fatal
        (that behavior is now test_measured_package_without_a_floor_is_fatal).
        This codebase has a documented history of test comments asserting
        things that are not true; a stale test NAME is the same defect.
        """
        p = profile((f"{PKG}/newpkg", "x.go", 80, 1), (f"{PKG}/newpkg", "x.go", 20, 0))
        code, _, err = self.gate(thresholds(f"{PKG}/newpkg 85.0\n"), [p])
        self.assertEqual(code, 1, "a brand-new package must still face the 85 floor")
        self.assertIn("newpkg", err)

    def test_floor_below_the_minimum_is_fatal(self):
        """A floor cannot be edited down below the 85% minimum.

        Regression: floors were parsed with float() and no bound, so
        `internal/engine 40.0` passed at 97.7% measured and exit 0. That was
        the single quiet edit by which ANY floor could be weakened — in the
        file whose entire purpose is preventing exactly that.
        """
        p = profile((f"{PKG}/engine", "apply.go", 97, 1), (f"{PKG}/engine", "apply.go", 3, 0))
        code, _, err = self.gate(thresholds(f"{PKG}/engine 40.0\n"), [p])
        self.assertEqual(code, 1)
        # The literal 85 also pins FLOOR itself: editing FLOOR down breaks
        # this assertion, so the constant cannot be weakened silently either.
        self.assertIn("85% minimum", err)

    def test_non_finite_floors_are_rejected_legibly(self):
        """float() accepts nan/inf; `nan < FLOOR` and `inf < FLOOR` are both False.

        Neither can make the gate PASS (pct >= nan and pct >= inf are both
        False), so this is about legibility, not safety: without the isfinite
        guard the message read "below its floor of nan%", sending a reader
        hunting a coverage regression that does not exist.
        """
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        for literal in ("nan", "NaN", "inf", "Infinity", "1e400"):
            with self.subTest(literal):
                code, _, err = self.gate(thresholds(f"{PKG}/engine {literal}\n"), [p])
                self.assertEqual(code, 1)
                self.assertIn("not a real percentage", err)
                self.assertNotIn("nan%", err)

    def test_every_bad_threshold_line_is_reported_at_once(self):
        """One round trip, not one per bad line — matching the set-equality fatals."""
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        t = thresholds(f"{PKG}/engine 40.0\n{PKG}/store 10.0\n{PKG}/rules garbage\n")
        code, _, err = self.gate(t, [p])
        self.assertEqual(code, 1)
        for expected in ("engine", "store", "rules"):
            self.assertIn(expected, err)

    def test_floor_exactly_at_the_minimum_is_allowed(self):
        p = profile((f"{PKG}/engine", "apply.go", 90, 1), (f"{PKG}/engine", "apply.go", 10, 0))
        code, _, _ = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p])
        self.assertEqual(code, 0, "85.0 is the minimum, not below it")

    # Merging is no longer this script's job: `go tool covdata merge` does it
    # natively. The union/idempotence/covermode tests that lived here were
    # deleted with the code they covered — see check-coverage.py's docstring
    # for why that code went.




    def test_untested_package_is_fatal(self):
        """A package that emits NO coverage data at all must fail the gate.

        Regression, and the sharpest one found: `go test -cover -args
        -test.gocoverdir` produces covdata only from test binaries that RUN, so
        a package with no _test.go files emits nothing. It then appears in
        neither `measured` nor `thresholds`, so NEITHER set-equality direction
        can see it — an untested package shipped with the gate reporting
        success. The older -coverprofile pipeline listed it at 0%, which is why
        this guard was (correctly, then) deleted as redundant and had to come
        back when the pipeline changed.
        """
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        t = thresholds(f"{PKG}/engine 85.0\n")
        e = expectfile(f"{PKG}/engine", f"{PKG}/untested")
        code, _, err = self.gate(t, [p], e)
        self.assertEqual(code, 1)
        self.assertIn("untested", err)
        self.assertIn("no coverage data", err)

    def test_missing_expected_file_is_fatal(self):
        p = profile((f"{PKG}/engine", "apply.go", 100, 1))
        code, _, err = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p],
                                 "/nonexistent/expected.txt")
        self.assertEqual(code, 1)
        self.assertIn("invisible to the gate", err)

    def test_missing_profile_is_fatal(self):
        code, _, err = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [])
        self.assertEqual(code, 1)
        self.assertIn("missing or empty", err)

    def test_all_packages_excluded_is_fatal_without_traceback(self):
        p = profile(("github.com/PatrikLager/vtt-platform/contract/gen/vtt/v1", "x.pb.go", 10, 0))
        code, _, err = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p])
        self.assertEqual(code, 1)
        self.assertIn("no measurable packages", err)

    def test_malformed_profile_line_is_fatal(self):
        """Both malformed shapes must be fatal, not silently skipped.

        The three-field case ("this is not a profile line" happens to split
        into exactly 3 under rsplit(" ", 2)) lands on the int() raise. The
        TWO-field case is the one that reaches the field-count check — and a
        round-2 mutation showed that branch was unreached: replacing its raise
        with `continue` survived, meaning a torn line would be dropped,
        understating a package's statement total and INFLATING its percentage.
        """
        for name, body in [
            ("three fields", "mode: set\nthis is not a profile line\n"),
            ("two fields", f"mode: set\n{PKG}/engine/apply.go:1.1,2.2 5\n"),
        ]:
            with self.subTest(name):
                fh = tempfile.NamedTemporaryFile("w", suffix=".out", delete=False)
                fh.write(body)
                fh.close()
                code, _, err = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [fh.name])
                self.assertEqual(code, 1)
                self.assertIn("malformed", err)

    # --- file paths containing spaces must not corrupt attribution ---

    def test_paths_with_spaces_parse_correctly(self):
        p = profile((f"{PKG}/engine", "a file.go", 100, 1))
        code, out, _ = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p])
        self.assertEqual(code, 0)
        self.assertIn(f"{PKG}/engine", out)

    # --- the ratchet reminds you to turn it ---

    def test_well_above_floor_suggests_a_raise(self):
        p = profile((f"{PKG}/engine", "apply.go", 98, 1), (f"{PKG}/engine", "apply.go", 2, 0))
        code, out, _ = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p])
        self.assertEqual(code, 0)
        self.assertIn("consider ratcheting", out)

    def test_just_above_floor_does_not_nag(self):
        """Run-to-run variance must not produce advice on every build."""
        p = profile((f"{PKG}/engine", "apply.go", 856, 1), (f"{PKG}/engine", "apply.go", 144, 0))
        code, out, _ = self.gate(thresholds(f"{PKG}/engine 85.0\n"), [p])
        self.assertEqual(code, 0)
        self.assertNotIn("consider ratcheting", out)


if __name__ == "__main__":
    unittest.main(verbosity=2)
