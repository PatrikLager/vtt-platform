#!/usr/bin/env python3
"""Boundary tests for check-doc-owner.py.

A gate ships with "passes when it should fail" holes unless its own boundary is
tested — the standing lesson from the coverage gate's five review rounds. These
drive the checker over synthetic trees and assert WHICH findings come back.

Run: python3 tools/check_doc_owner_test.py
"""

import importlib.util
import io
import os
import pathlib
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

_spec = importlib.util.spec_from_file_location(
    "check_doc_owner",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-doc-owner.py"))
cdo = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cdo)


def tree(**files):
    """A temp dir of .go files; returns its path (caller cleans up)."""
    d = tempfile.mkdtemp()
    for name, body in files.items():
        p = pathlib.Path(d) / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")
    return d


def run(root):
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = cdo.main(["check-doc-owner.py", root])
    return code, out.getvalue(), err.getvalue()


class DocOwnerTest(unittest.TestCase):
    def test_a_comment_left_behind_by_an_inserted_function_is_caught(self):
        """The defect, exactly as it appeared twice in internal/harness/soak.go."""
        root = tree(**{"a.go": '''package p

// alpha does the alpha thing.
func beta() {}

func alpha() {}
'''})
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("above `beta`", err)
        self.assertIn("describing `alpha`", err)
        # The advice must be "move it back", not "rewrite it": the comment is
        # correct prose about a real function, just in the wrong place.
        self.assertIn("move the comment back", err)

    def test_a_correctly_placed_comment_passes(self):
        """The control. A gate that flags correct code gets switched off."""
        root = tree(**{"a.go": '''package p

// alpha does the alpha thing.
func alpha() {}

// beta does the beta thing.
func beta() {}
'''})
        code, out, _ = run(root)
        self.assertEqual(code, 0)
        self.assertIn("sits on its own function", out)

    def test_prose_that_merely_starts_with_a_word_is_not_flagged(self):
        """NARROW ON PURPOSE. Only a word that NAMES A REAL FUNCTION counts.

        internal/rules has deliberate `// Complexity: ...` blocks above
        `//nolint:gocyclo`. Demanding those be rewritten would be a gate whose
        first act is to break correct code, which is how gates get disabled.
        """
        root = tree(**{"a.go": '''package p

// Complexity: straight-line validation, splitting it would only move branches.
//
//nolint:gocyclo
func alpha() {}

// Deliberately unexported: nothing outside this file may call it.
func beta() {}
'''})
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    def test_a_function_with_no_doc_is_not_a_finding(self):
        """Missing docs are a different argument, and not this gate's."""
        root = tree(**{"a.go": "package p\n\nfunc alpha() {}\n"})
        self.assertEqual(run(root)[0], 0)

    def test_methods_are_checked_too(self):
        """A receiver must not hide the defect — most of this repo is methods."""
        root = tree(**{"a.go": '''package p

// alpha reports something.
func (s *S) beta() {}

func (s *S) alpha() {}
'''})
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("above `beta`", err)

    def test_a_function_named_in_another_file_still_counts(self):
        """The comment's owner is usually elsewhere by the time this is noticed."""
        root = tree(**{
            "a.go": "package p\n\n// alpha does the alpha thing.\nfunc beta() {}\n",
            "b.go": "package p\n\nfunc alpha() {}\n",
        })
        self.assertEqual(run(root)[0], 1)

    def test_test_files_and_generated_code_are_skipped(self):
        """Both are noisy and neither is where this defect matters."""
        root = tree(**{
            "a_test.go": "package p\n\n// alpha does it.\nfunc beta() {}\n\nfunc alpha() {}\n",
            "gen/c.go": "package p\n\n// alpha does it.\nfunc beta() {}\n\nfunc alpha() {}\n",
        })
        self.assertEqual(run(root)[0], 0)

    def test_the_real_tree_is_clean(self):
        """The gate must pass on this repo, or it is not a gate."""
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        code, _, err = run(os.path.join(repo, "internal"))
        self.assertEqual(code, 0, err)


if __name__ == "__main__":
    unittest.main(verbosity=1)
