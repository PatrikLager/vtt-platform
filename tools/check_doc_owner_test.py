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
        # Both causes, because the gate cannot tell them apart from the outside
        # and prose that picks one sends the reader to the wrong file.
        self.assertIn("move the comment back", err)
        self.assertIn("open by naming", err)

    def test_the_orphan_is_named_when_the_newcomer_brought_its_own_doc(self):
        """soak.go's REAL shape: two docs, one block, the orphan on top.

        The single-doc case above does not exercise this. Reading only the line
        directly above the func, or taking the LAST line of the block, both
        still pass that test and both miss this — which is the shape the gate
        was written for. The finding must name the FIRST doc, not the second.
        """
        root = tree(**{"a.go": '''package p

// alpha reports whether the log grew.
//
// It exists because a quiet connection and a wedged one look identical.
// beta drains whatever the catch-up left behind.
func beta() {}

func alpha() {}
'''})
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("describing `alpha`", err)

    def test_a_comment_orphaned_by_a_blank_line_documents_nothing(self):
        """The commoner half of the defect, and it was live in internal/gateway.

        Insert a documented function and gofmt does NOT join the two comment
        blocks — you get `orphan doc / BLANK / newcomer's doc / func`. Go then
        attaches the orphan to nothing at all: `go doc` prints no comment for
        either function, and the rationale is silently unreachable. This was
        sitting on EncodeFrame in internal/gateway/codec.go, ten lines of
        data-race rationale attached to nothing, while the gate ran green.

        A file with no trailing newline covers the same rule's other end: there
        is no blank line after the block because there is nothing after it at
        all. gofmt would add the newline, but a linter reads what is on disk.
        """
        root = tree(**{
            "a.go": '''package p

// alpha marshals the frame.
// It matters because the seam used to be a global.

// beta does the beta thing.
func beta() {}

func alpha() {}
''',
            "b.go": "package p\n\nfunc gamma() {}\n\n// alpha marshals the frame.",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("documents NOTHING", err)
        self.assertIn("a.go:3", err)
        self.assertIn("b.go:5", err)

    def test_a_blank_line_orphan_naming_nothing_real_is_left_alone(self):
        """The control for the blank-line rule.

        Standalone commentary above a blank line is ordinary Go — file section
        headers, package prose, a note before an import block. Only a first
        word that NAMES A REAL FUNCTION makes it an orphaned doc.
        """
        root = tree(**{"a.go": '''package p

// Wire format notes: everything below is protojson.

// alpha does the alpha thing.
func alpha() {}
'''})
        self.assertEqual(run(root)[0], 0)

    def test_a_comment_inside_a_function_body_is_not_a_doc(self):
        """Indented comments are never doc comments, and blank lines follow them
        constantly. Scanning them would flag ordinary code the moment somebody
        wrote a step comment naming the helper it is about to call."""
        root = tree(**{"a.go": '''package p

func beta() {
	// alpha is what we are about to call.

	alpha()
}

func alpha() {}
'''})
        self.assertEqual(run(root)[0], 0)

    def test_an_adjudicated_block_is_honoured(self):
        """Rule 2: never weaken a gate to pass it. The only alternative to an
        escape hatch is rewriting correct prose, so the hatch has to exist and
        has to carry a reason — the same shape as //nolint: and nosemgrep.

        The hatch sits BELOW the prose on purpose. Written as the first line it
        would change which word the gate reads, and this test would pass with
        no hatch implemented at all — which is how it was first written here.
        The opening word is a bare `alpha` for the same reason: `alpha's` does
        not survive word extraction, so it too passed with the hatch disabled.
        """
        root = tree(**{"a.go": '''package p

// alpha is the counterpart of this one, which runs on the way out.
//
//doc-owner:ok deliberate cross-reference, alpha is named on purpose
func beta() {}

func alpha() {}
'''})
        self.assertEqual(run(root)[0], 0)

    def test_the_named_word_is_read_through_markup_and_a_bare_lead(self):
        """How the first word is extracted, which is the whole rule's hinge.

        Two shapes that look like nothing and disable the gate silently: a
        block opening with a bare `//` (take block[0] literally and the word is
        empty, so the block is skipped whole), and a name wearing this repo's
        habitual backticks (skip the strip and `alpha` never equals alpha).
        """
        root = tree(**{"a.go": '''package p

//
// `alpha` does the alpha thing.
func beta() {}

func alpha() {}
'''})
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("describing `alpha`", err)

    def test_an_empty_scan_is_a_failure_not_a_pass(self):
        """A gate that reports success over zero files is how check:mutation
        exited 0 with the disk full. Nothing scanned is nothing proven."""
        code, _, err = run(os.path.join(tempfile.mkdtemp(), "not-a-directory"))
        self.assertEqual(code, 2)
        self.assertIn("no Go files", err)

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
        """Skipped for PRECISION, not because the defect cannot happen there.

        It does happen there — two live instances sat in gateway's own test
        files. But run over this repo's tests the rule is ~25% precise: test
        helpers cross-reference each other constantly, and six of eight hits
        were correct prose. A gate that is wrong three times in four gets
        switched off. Revisit if the hatch above proves cheap to apply.
        """
        root = tree(**{
            "a_test.go": "package p\n\n// alpha does it.\nfunc beta() {}\n\nfunc alpha() {}\n",
            "gen/c.go": "package p\n\n// alpha does it.\nfunc beta() {}\n\nfunc alpha() {}\n",
            # A scannable file, or the empty-scan guard exits 2 and this passes
            # without ever reaching the skip logic.
            "real.go": "package p\n\n// gamma does it.\nfunc gamma() {}\n",
        })
        self.assertEqual(run(root)[0], 0)

    def test_the_real_tree_is_clean(self):
        """The gate must pass on this repo, or it is not a gate.

        Asserting the FILE COUNT, not just the exit code: this same assertion
        passed in 0.000s against a path that did not exist, because rglob on a
        missing directory yields nothing and a clean scan of nothing exits 0.
        Move this tool one directory and the vacuum comes back.
        """
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        code, out, err = run(repo)
        self.assertEqual(code, 0, err)
        self.assertRegex(out, r"check:doc-owner: ([4-9]\d|\d{3,}) files")


if __name__ == "__main__":
    unittest.main(verbosity=1)
