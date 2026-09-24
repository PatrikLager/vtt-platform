"""Boundary tests for check-comments.py, the comment gate SPEC-010 describes.

Each test builds a small git repository with a `main` branch, copies the gate
and the prose gate it imports into it, changes the working tree, and runs the
gate against `main`. One case per refusal and per pass. A ledger row is set
from the share the fixture will have, so that only the case under test fires.

Run: python3 tools/check_comments_test.py
"""
import math
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent
TOOL = TOOLS / "check-comments.py"
TOOL_BYTES = TOOL.read_bytes()  # absent until the checker exists: red at import
PROSE_BYTES = (TOOLS / "check-new-prose.py").read_bytes()

LEDGER = "tools/comment-ceilings.txt"
GO_CLEAN = "package a\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int {\n\treturn 2\n}\n\nfunc C() int {\n\treturn 3\n}\n"
GO_BIG = "package a\n" + "".join(f"\nfunc F{i}() int {{\n\treturn {i}\n}}\n" for i in range(6))


def git(repo, *args):
    return subprocess.run(["git", "-C", str(repo), *args],
                          capture_output=True, text=True, check=True).stdout


def share(text):
    lines = [l for l in text.split("\n") if l.strip()]
    return 100.0 * sum(1 for l in lines if l.strip().startswith("//")) / len(lines)


def ceil1(x):
    return math.ceil(x * 10 - 1e-9) / 10


class CommentGateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Path(self.tmp.name)
        git(self.repo, "init", "-q")
        git(self.repo, "config", "user.email", "t@example.com")
        git(self.repo, "config", "user.name", "t")
        (self.repo / "tools").mkdir()
        (self.repo / "tools" / "check-comments.py").write_bytes(TOOL_BYTES)
        (self.repo / "tools" / "check-new-prose.py").write_bytes(PROSE_BYTES)

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, name, text):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text, encoding="utf-8")

    def ledger(self, rows):
        body = "# comment ceilings, SPEC-010\n" + "".join(f"{p}  {v:.1f}\n" for p, v in rows)
        self.write(LEDGER, body)

    def base(self, files, ledger_rows=None):
        """Commit files (and a ledger) as `main`, then branch `work`."""
        for name, text in files.items():
            self.write(name, text)
        if ledger_rows is not None:
            self.ledger(ledger_rows)
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", "base")
        git(self.repo, "branch", "-M", "main")
        git(self.repo, "checkout", "-q", "-b", "work")

    def run_gate(self, *args):
        return subprocess.run([sys.executable, "-B", "tools/check-comments.py", *(args or ("main",))],
                              cwd=str(self.repo), capture_output=True, text=True)

    def assertRefused(self, got, *needles):
        text = got.stdout + got.stderr
        self.assertEqual(got.returncode, 1, text)
        for n in needles:
            self.assertIn(n, text)

    def assertClean(self, got):
        text = got.stdout + got.stderr
        self.assertEqual(got.returncode, 0, text)
        self.assertRegex(got.stdout.strip().splitlines()[-1],
                         r"^check:comments: \d+ files, \d+ added comment lines, \d+ ledger rows; clean")

    def change(self, base_text, new_text, path="internal/a/a.go"):
        """A base at the share the change will reach, then the change."""
        self.base({path: base_text}, [(path, ceil1(share(new_text)))])
        self.write(path, new_text)

    # --- banned terms on added lines --------------------------------------

    # VTT-050
    def test_an_added_line_with_a_banned_term_is_refused(self):
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", "// measured 2026-01-01: 35 of 40 trials\nfunc B()"))
        self.assertRefused(self.run_gate(), "internal/a/a.go:7", "measured")

    # VTT-050
    def test_a_banned_term_in_the_base_is_not_this_changes_fault(self):
        old = GO_CLEAN.replace("func B()", "// measured 2026-01-01: 35 of 40 trials\nfunc B()")
        self.change(old, old.replace("func C()", "// Hold the lock first.\nfunc C()"))
        self.assertClean(self.run_gate())

    # VTT-050
    def test_a_docs_path_carrying_a_date_is_not_refused(self):
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", "// See docs/reports/2026-09-24-joining-record-and-code.md.\nfunc B()"))
        self.assertClean(self.run_gate())

    # VTT-050
    def test_a_directive_is_not_refused(self):
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", "//go:generate echo x\n//nolint:gocyclo\nfunc B()"))
        self.assertClean(self.run_gate())

    # VTT-050 VTT-053
    def test_a_warning_comment_passes_with_the_completion_line(self):
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", "// Hold mu before calling; SPEC-009 says why.\nfunc B()"))
        got = self.run_gate()
        self.assertClean(got)
        self.assertIn("1 added comment lines", got.stdout)

    # --- blocks ------------------------------------------------------------

    # VTT-052
    def test_a_block_over_the_bound_that_the_change_touches_is_refused(self):
        block = "".join(f"// line {i} of a warning that runs long\n" for i in range(7))
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", block + "func B()"))
        self.assertRefused(self.run_gate(), "internal/a/a.go:7", "block", "7 lines")

    # VTT-052
    def test_one_line_added_to_a_legacy_block_over_the_bound_is_refused(self):
        block = "".join(f"// legacy line {i}\n" for i in range(8))
        old = GO_CLEAN.replace("func B()", block + "func B()")
        self.change(old, old.replace("// legacy line 7\n", "// legacy line 7\n// one more\n"))
        self.assertRefused(self.run_gate(), "internal/a/a.go", "block", "9 lines")

    # VTT-052
    def test_a_blank_line_does_not_end_a_block(self):
        block = "".join(f"// part one {i}\n" for i in range(4)) + "\n" + "".join(f"// part two {i}\n" for i in range(4))
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", block + "func B()"))
        self.assertRefused(self.run_gate(), "block", "8 lines")

    # VTT-052
    def test_a_block_of_exactly_the_bound_passes(self):
        block = "".join(f"// line {i}\n" for i in range(6))
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", block + "func B()"))
        self.assertClean(self.run_gate())

    # VTT-052
    def test_the_go_package_doc_is_exempt(self):
        doc = "".join(f"// Package a, line {i} of its doc.\n" for i in range(8))
        self.change(GO_CLEAN, doc + GO_CLEAN)
        self.assertClean(self.run_gate())

    # VTT-052
    def test_a_ts_opening_block_is_not_exempt(self):
        head = "/*\n" + "".join(f" * line {i}\n" for i in range(7)) + " */\n"
        self.base({"client/src/a.ts": "export const a = 1;\n"}, [("client/src/a.ts", 90.0)])
        self.write("client/src/a.ts", head + "export const a = 1;\n")
        self.assertRefused(self.run_gate(), "client/src/a.ts", "block", "9 lines")

    # VTT-052
    def test_a_go_line_starting_with_a_star_is_code_not_comment(self):
        code = "package a\n\ntype U struct{ n int }\n"
        self.change(code, code + "\nfunc Reset(u *U) {\n\t*u = U{}\n}\n")
        got = self.run_gate()
        self.assertClean(got)
        self.assertIn("0 added comment lines", got.stdout)

    # VTT-050
    def test_a_go_block_comment_is_read_line_by_line(self):
        self.change(GO_CLEAN, GO_CLEAN.replace("func B()", "/*\nThis was measured 2026-01-01.\n*/\nfunc B()"))
        self.assertRefused(self.run_gate(), "internal/a/a.go:8", "measured")

    # --- ceilings ----------------------------------------------------------

    # VTT-053
    def test_a_comment_line_added_above_the_ceiling_is_refused(self):
        old = GO_CLEAN.replace("func B()", "// B is a number.\nfunc B()")
        top = ceil1(share(old))
        self.base({"internal/a/a.go": old}, [("internal/a/a.go", top)])
        self.write("internal/a/a.go", old.replace("func C()", "// C is a number.\nfunc C()"))
        self.assertRefused(self.run_gate(), "internal/a/a.go", f"{top:.1f}", "ceiling")

    # VTT-053
    def test_a_rise_from_removed_code_alone_is_a_notice_not_a_refusal(self):
        old = GO_CLEAN.replace("func B()", "// B is a number.\nfunc B()")
        self.base({"internal/a/a.go": old}, [("internal/a/a.go", ceil1(share(old)))])
        self.write("internal/a/a.go", old.replace("\nfunc C() int {\n\treturn 3\n}\n", ""))
        got = self.run_gate()
        self.assertClean(got)
        self.assertIn("internal/a/a.go", got.stdout)
        self.assertIn("notice", got.stdout)

    # VTT-054
    def test_a_ledger_row_raised_above_the_base_is_refused(self):
        self.base({"internal/a/a.go": GO_CLEAN}, [("internal/a/a.go", 0.0)])
        self.ledger([("internal/a/a.go", 0.1)])
        self.assertRefused(self.run_gate(), "internal/a/a.go", "0.1", "0.0", "raised")

    # VTT-054
    def test_write_ledger_never_raises_a_row(self):
        old = GO_CLEAN.replace("func B()", "// B is a number.\nfunc B()")
        self.base({"internal/a/a.go": old}, [("internal/a/a.go", 5.0)])
        got = self.run_gate("--write-ledger")
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertIn("internal/a/a.go  5.0\n", (self.repo / LEDGER).read_text())

    # VTT-055
    def test_a_share_fallen_more_than_the_band_under_its_ceiling_is_refused(self):
        old = GO_CLEAN.replace("func B()", "// B is a number.\n// It is two.\nfunc B()")
        self.base({"internal/a/a.go": old}, [("internal/a/a.go", ceil1(share(old)))])
        self.write("internal/a/a.go", GO_CLEAN)
        self.assertRefused(self.run_gate(), "internal/a/a.go", "--write-ledger")

    # VTT-055
    def test_a_share_within_the_band_passes(self):
        old = GO_BIG.replace("func F1()", "// One.\n// Two.\n// Three.\n// Four.\nfunc F1()")
        self.base({"internal/a/a.go": old}, [("internal/a/a.go", ceil1(share(old)))])
        self.write("internal/a/a.go", old + "\nvar x = 1\n")
        self.assertClean(self.run_gate())

    # VTT-056
    def test_a_row_whose_file_is_gone_is_refused(self):
        self.base({"internal/a/a.go": GO_CLEAN, "internal/a/b.go": GO_CLEAN.replace("package a", "package a\n")},
                  [("internal/a/a.go", 0.0), ("internal/a/b.go", 0.0)])
        (self.repo / "internal/a/b.go").unlink()
        self.assertRefused(self.run_gate(), "internal/a/b.go", "stale")

    # VTT-056
    def test_write_ledger_drops_a_stale_row(self):
        self.base({"internal/a/a.go": GO_CLEAN, "internal/a/b.go": GO_CLEAN.replace("package a", "package a\n")},
                  [("internal/a/a.go", 0.0), ("internal/a/b.go", 0.0)])
        (self.repo / "internal/a/b.go").unlink()
        got = self.run_gate("--write-ledger")
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertNotIn("internal/a/b.go", (self.repo / LEDGER).read_text())

    # VTT-056
    def test_a_git_mv_is_a_stale_row_until_write_ledger_moves_it(self):
        text = GO_CLEAN.replace("func B()", "// One.\n// Two.\n// Three.\n// Four.\nfunc B()")
        self.base({"internal/a/a.go": text}, [("internal/a/a.go", ceil1(share(text)))])
        git(self.repo, "mv", "internal/a/a.go", "internal/a/moved.go")
        self.assertRefused(self.run_gate(), "stale row for internal/a/a.go", "moves it")
        got = self.run_gate("--write-ledger")
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertIn("internal/a/moved.go  %.1f\n" % ceil1(share(text)), (self.repo / LEDGER).read_text())
        self.assertNotIn("internal/a/a.go ", (self.repo / LEDGER).read_text())
        self.assertClean(self.run_gate())

    # VTT-056 VTT-057
    def test_write_ledger_refuses_to_strand_a_plainly_moved_file(self):
        text = GO_CLEAN.replace("func B()", "// One.\n// Two.\n// Three.\n// Four.\nfunc B()")
        self.base({"internal/a/a.go": text}, [("internal/a/a.go", ceil1(share(text)))])
        (self.repo / "internal/a/a.go").rename(self.repo / "internal/a/moved.go")
        got = self.run_gate("--write-ledger")
        self.assertEqual(got.returncode, 1, got.stdout + got.stderr)
        self.assertIn("git mv", got.stdout)
        self.assertIn("internal/a/a.go  %.1f\n" % ceil1(share(text)), (self.repo / LEDGER).read_text())

    # VTT-057
    def test_a_new_file_above_the_default_ceiling_with_no_row_is_refused(self):
        self.base({"internal/a/a.go": GO_CLEAN}, [("internal/a/a.go", 0.0)])
        self.write("internal/a/new.go", "package a\n\n// One.\n// Two.\n// Three.\n// Four.\nfunc N() int {\n\treturn 1\n}\n")
        self.assertRefused(self.run_gate(), "internal/a/new.go", "25.0", "default")

    # VTT-057
    def test_a_new_file_under_the_default_ceiling_passes(self):
        self.base({"internal/a/a.go": GO_CLEAN}, [("internal/a/a.go", 0.0)])
        self.write("internal/a/new.go", "package a\n\n// N is one.\nfunc N() int {\n\treturn 1\n}\n")
        self.assertClean(self.run_gate())

    # --- the run itself ----------------------------------------------------

    # VTT-058
    def test_a_run_that_scans_nothing_fails(self):
        self.base({"README.md": "nothing in scope\n"}, [])
        got = self.run_gate()
        self.assertEqual(got.returncode, 2, got.stdout + got.stderr)
        self.assertIn("nothing was scanned", got.stdout + got.stderr)

    # VTT-058
    def test_a_missing_base_ref_fails(self):
        self.base({"internal/a/a.go": GO_CLEAN}, [("internal/a/a.go", 0.0)])
        got = self.run_gate("no-such-branch")
        self.assertNotEqual(got.returncode, 0)
        self.assertIn("no-such-branch", got.stdout + got.stderr)

    # VTT-058
    def test_a_missing_ledger_fails(self):
        self.base({"internal/a/a.go": GO_CLEAN})
        got = self.run_gate()
        self.assertEqual(got.returncode, 2, got.stdout + got.stderr)
        self.assertIn(LEDGER, got.stdout + got.stderr)

    # VTT-058
    def test_a_ledger_value_that_is_not_a_share_fails(self):
        self.base({"internal/a/a.go": GO_CLEAN})
        for bad in ("nan", "-5.0", "50.05", "101.0", "50"):
            self.write(LEDGER, "internal/a/a.go  %s\n" % bad)
            got = self.run_gate()
            self.assertEqual(got.returncode, 2, bad + ": " + got.stdout + got.stderr)
            self.assertIn(LEDGER, got.stdout + got.stderr)

    # VTT-058
    def test_a_ledger_naming_a_path_twice_fails(self):
        self.base({"internal/a/a.go": GO_CLEAN})
        self.write(LEDGER, "internal/a/a.go  0.0\ninternal/a/a.go  50.0\n")
        got = self.run_gate()
        self.assertEqual(got.returncode, 2, got.stdout + got.stderr)
        self.assertIn("second time", got.stdout + got.stderr)

    # VTT-054
    def test_no_ledger_at_the_base_skips_the_raise_check_and_says_so(self):
        self.base({"internal/a/a.go": GO_CLEAN})
        self.ledger([("internal/a/a.go", 0.0)])
        got = self.run_gate()
        self.assertClean(got)
        self.assertIn("no ledger at main", got.stdout)


if __name__ == "__main__":
    unittest.main()
