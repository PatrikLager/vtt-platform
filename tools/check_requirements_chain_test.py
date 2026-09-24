#!/usr/bin/env python3
"""Boundary tests for check-requirements-chain.py.

A gate ships with "passes when it should fail" holes unless its own boundary is
tested. These drive the checker over synthetic repositories and assert WHICH
findings come back, one case per refusal and per pass.

Fixtures carry the tag TT, never the project's own: the checker scans this file
too, and a VTT id written here as fixture data would be read as a citation.

Run: python3 tools/check_requirements_chain_test.py
"""

import importlib.util
import io
import os
import pathlib
import re
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

_spec = importlib.util.spec_from_file_location(
    "check_requirements_chain",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-requirements-chain.py"))
crc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(crc)

HEADER = "| Id | Requirement | Verified by |\n|---|---|---|\n"


def register(rows, tag="TT", header=HEADER):
    """A register with a project line, prose, the table, and prose below it."""
    body = "# Requirements\n\n"
    if tag is not None:
        body += f"project: {tag}\n\n"
    body += "Prose above the table.\n\n" + header
    for row in rows:
        body += row + "\n"
    body += "\nProse below the table, which is not a row.\n"
    return body


def tree(**files):
    """A temp dir laid out like the repository; returns its path."""
    d = tempfile.mkdtemp()
    for name, body in files.items():
        p = pathlib.Path(d) / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")
    return d


def run(root):
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = crc.main(["check-requirements-chain.py", root])
    return code, out.getvalue(), err.getvalue()


GO_TEST = "package p\n\n// TT-001\nfunc TestAlpha(t *testing.T) {}\n"


def clean_fixture():
    """One row, one Go test citing it, one specification naming it."""
    return tree(**{
        "docs/requirements.md": register(
            ["| TT-001 | Alpha holds. | internal/p/alpha_test.go#TestAlpha |"]),
        "docs/specifications/001-alpha.md": "# SPEC-001\n\n## Requirements\n\nTT-001.\n",
        "internal/p/alpha_test.go": GO_TEST,
    })


class RequirementsChainTest(unittest.TestCase):

    # --- citations that resolve to nothing -----------------------------------

    def test_a_test_citing_an_id_no_row_defines_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register([]),
            "internal/p/alpha_test.go": "package p\n\n// TT-999\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("alpha_test.go", err)
        self.assertIn("TT-999", err)

    def test_a_specification_naming_an_id_no_row_defines_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register([]),
            "docs/specifications/001-alpha.md": "# SPEC-001\n\nThis record carries TT-999.\n",
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("001-alpha.md", err)
        self.assertIn("TT-999", err)

    # --- evidence that does not hold -----------------------------------------

    def test_evidence_naming_a_missing_file_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | internal/p/nowhere_test.go#TestAlpha |"]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("TT-001", err)
        self.assertIn("nowhere_test.go", err)

    def test_evidence_naming_a_file_that_does_not_carry_the_id_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | internal/p/alpha_test.go#TestAlpha |"]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("TT-001", err)
        self.assertIn("does not carry", err)

    def test_evidence_naming_a_check_that_is_not_in_the_file_is_refused(self):
        """Three languages, three shapes of a check's declaration."""
        for name, body in {
            "internal/p/alpha_test.go": "package p\n\n// TT-001\nfunc TestBeta(t *testing.T) {}\n",
            "tools/alpha_test.py": "# TT-001\ndef test_beta():\n    pass\n",
            "client/test/alpha.test.ts": '// TT-001\ntest("beta holds", () => {});\n',
        }.items():
            root = tree(**{
                "docs/requirements.md": register(
                    [f"| TT-001 | Alpha holds. | {name}#TestAlpha |"]),
                name: body,
                # A Go test file somewhere, so the empty-scan guard never fires.
                "internal/q/q_test.go": "package q\n\nfunc TestQ(t *testing.T) {}\n",
            })
            code, _, err = run(root)
            self.assertEqual(code, 1, name)
            self.assertIn("TestAlpha", err)
            self.assertIn("no check named", err)

    def test_a_check_declared_in_each_language_is_found(self):
        """The control for the case above: every declaration shape resolves."""
        root = tree(**{
            "docs/requirements.md": register([
                "| TT-001 | Alpha holds. | internal/p/alpha_test.go#TestAlpha |",
                "| TT-002 | Beta holds. | tools/beta_test.py#test_beta |",
                "| TT-003 | Gamma holds. | client/test/gamma.test.ts#gamma holds |",
                "| TT-004 | Delta holds. | internal/p/alpha_test.go#TestDelta |",
            ]),
            "internal/p/alpha_test.go": (
                "package p\n\n// TT-001\nfunc TestAlpha(t *testing.T) {}\n\n"
                "// TT-004\nfunc (s *S) TestDelta(t *testing.T) {}\n"),
            "tools/beta_test.py": "# TT-002\ndef test_beta():\n    pass\n",
            "client/test/gamma.test.ts": '// TT-003\ntest("gamma holds", () => {});\n',
        })
        code, out, err = run(root)
        self.assertEqual(code, 0, err)
        self.assertIn("4 rows", out)

    def test_an_evidence_entry_without_a_check_name_is_refused(self):
        """A path alone is not evidence: rule 8 wants the name, and a bare path
        is how a sentence in the cell would otherwise be read."""
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | internal/p/alpha_test.go |"]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("names no check", err)

    def test_a_sentence_in_the_evidence_cell_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | Two tests aim at this and neither can see it. |"]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        self.assertEqual(run(root)[0], 1)

    # --- the two answers that are not paths ----------------------------------

    def test_open_and_a_named_reading_pass(self):
        root = tree(**{
            "docs/requirements.md": register([
                "| TT-001 | Alpha holds. | **OPEN — no test yet** |",
                "| TT-002 | Beta holds. | **READING — Phase 4b** |",
            ]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, out, err = run(root)
        self.assertEqual(code, 0, err)
        self.assertIn("2 rows", out)

    def test_a_reading_that_names_no_review_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | **READING —** |"]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("names no review", err)

    def test_a_blank_evidence_cell_is_refused(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. |  |"]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("blank", err)

    # --- the row's id ----------------------------------------------------------

    def test_a_row_id_of_another_shape_is_refused(self):
        """A per-subject series is what the dispenser cannot see."""
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-VIS-001 | Alpha holds. | **OPEN — no test yet** |"]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("TT-VIS-001", err)
        self.assertIn("not of the shape", err)

    def test_a_row_numbered_zero_is_refused(self):
        """The dispenser counts from one; a hand-typed zero is not an allocation."""
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-000 | Alpha holds. | **OPEN — no test yet** |"]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("TT-000", err)

    def test_two_rows_sharing_an_id_are_refused(self):
        root = tree(**{
            "docs/requirements.md": register([
                "| TT-001 | Alpha holds. | **OPEN — no test yet** |",
                "| TT-001 | Alpha holds again. | **OPEN — no test yet** |",
            ]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("twice", err)

    def test_markup_around_a_row_id_is_read_through(self):
        """The dispenser strips backticks, bold and underscores when it counts;
        the gate reads the same way, or a decorated row is a phantom."""
        root = tree(**{
            "docs/requirements.md": register([
                "| `TT-001` | Alpha holds. | internal/p/alpha_test.go#TestAlpha |",
                "| **TT-002** | Beta holds. | **OPEN — no test yet** |",
            ]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, out, err = run(root)
        self.assertEqual(code, 0, err)
        self.assertIn("2 rows", out)

    def test_an_escaped_separator_in_a_sentence_does_not_split_the_row(self):
        """The dispenser writes `\\|` for a `|` in the sentence."""
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha \\| beta holds. | internal/p/alpha_test.go#TestAlpha |"]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    # --- citations -------------------------------------------------------------

    def test_two_ids_on_one_comment_line_both_resolve(self):
        root = tree(**{
            "docs/requirements.md": register([
                "| TT-001 | Alpha holds. | internal/p/alpha_test.go#TestAlpha |",
                "| TT-002 | Beta holds. | internal/p/alpha_test.go#TestAlpha |",
            ]),
            "internal/p/alpha_test.go": "package p\n\n// TT-001 TT-002\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    def test_a_comma_inside_a_typescript_name_is_part_of_the_name(self):
        """283 client test names carry a comma; the separator is a comma that
        begins the next path#Check, and nothing else."""
        root = tree(**{
            "docs/requirements.md": register([
                "| TT-001 | Alpha holds. | client/test/g.test.ts#gamma, then delta, internal/p/alpha_test.go#TestAlpha |",
            ]),
            "client/test/g.test.ts": '// TT-001\ntest("gamma, then delta", () => {});\n',
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    def test_a_row_with_fewer_than_three_cells_is_refused_as_malformed(self):
        root = tree(**{
            "docs/requirements.md": register(
                ["| TT-001 | Alpha holds. | internal/p/alpha_test.go#TestAlpha"]),
            "internal/p/alpha_test.go": GO_TEST,
        })
        code, _, err = run(root)
        self.assertEqual(code, 1)
        self.assertIn("fewer than three cells", err)

    def test_another_projects_tag_is_not_read_as_a_claim(self):
        """This file's own fixtures, seen from the real register: a TT id in a
        VTT-tagged tree is data, not a citation, and the reverse."""
        root = tree(**{
            "docs/requirements.md": register([]),
            "internal/p/alpha_test.go": "package p\n\n// XX-999 DS-001\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    def test_test_files_outside_the_scanned_roots_and_generated_trees_are_skipped(self):
        root = tree(**{
            "docs/requirements.md": register([]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
            "internal/p/gen/g_test.go": "package g\n\n// TT-999\nfunc TestG(t *testing.T) {}\n",
            "client/node_modules/x/x.test.ts": "// TT-999\n",
            "docs/superpowers/specs/t_test.go": "// TT-999\n",
            "internal/p/notatest.go": "package p\n\n// TT-999\nfunc alpha() {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 0, err)

    # --- nothing scanned is nothing proven --------------------------------------

    def test_a_missing_register_is_a_failure_not_a_pass(self):
        root = tree(**{"internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n"})
        code, _, err = run(root)
        self.assertEqual(code, 2)
        self.assertIn("no register", err)

    def test_a_register_with_no_tag_is_a_failure(self):
        root = tree(**{
            "docs/requirements.md": register([], tag=None),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 2)
        self.assertIn("project:", err)

    def test_a_register_with_no_requirements_header_is_a_failure(self):
        root = tree(**{
            "docs/requirements.md": register([], header="| Id | Where |\n|---|---|\n"),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, _, err = run(root)
        self.assertEqual(code, 2)
        self.assertIn("header", err)

    def test_a_tree_with_no_test_files_is_a_failure(self):
        root = tree(**{"docs/requirements.md": register([]), "internal/p/p.go": "package p\n"})
        code, _, err = run(root)
        self.assertEqual(code, 2)
        self.assertIn("no test files", err)

    def test_an_empty_table_passes_and_says_so(self):
        """An empty register is a step, not a defect."""
        root = tree(**{
            "docs/requirements.md": register([]),
            "internal/p/alpha_test.go": "package p\n\nfunc TestAlpha(t *testing.T) {}\n",
        })
        code, out, err = run(root)
        self.assertEqual(code, 0, err)
        self.assertIn("0 rows", out)

    def test_a_clean_fixture_passes_and_prints_the_completion_line(self):
        code, out, err = run(clean_fixture())
        self.assertEqual(code, 0, err)
        self.assertRegex(out, r"check:requirements-chain: 1 rows?, 1 test files?, 1 specifications?")

    def test_the_real_tree_is_clean(self):
        """The gate must pass on this repository, or it is not a gate.

        The counts are FLOORS, not figures: a clean scan of nothing exits 0,
        so the assertion is that something was scanned."""
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        code, out, err = run(repo)
        self.assertEqual(code, 0, err)
        m = re.search(r"(\d+) rows?, (\d+) test files?, (\d+) specifications?", out)
        self.assertIsNotNone(m, out)
        self.assertGreaterEqual(int(m.group(2)), 100)
        self.assertGreaterEqual(int(m.group(3)), 2)


if __name__ == "__main__":
    unittest.main(verbosity=1)
