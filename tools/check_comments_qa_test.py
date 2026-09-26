"""Independent QA for tools/check-comments.py against SPEC-010, VTT-050..059.

Black-box: every test builds a small git repository, copies the gate and the
module it imports into that repository's tools/, commits a base on main,
changes the working tree, and runs the gate there. No mocks.

BOUND, DEFAULT and BAND are the values the gate's usage text states.
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from fractions import Fraction

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = "tools/check-comments.py"
BOUND = 6
DEFAULT = 25.0
BAND = 1.0
BANNED_WHY = r"an added comment line carries"
BLOCK_WHY = r"comment block of \d+ lines is over the bound"
CEIL_WHY = r"is above its ceiling [\d.]+ and this change added a comment line"
RAISE_WHY = r"raised to [\d.]+ above the base's"
NEWROW_WHY = r"has no row at the base and is above the default"
STALE_WHY = r"stale row for \S+, which does not exist"
BAND_WHY = r"has fallen more than [\d.]+ under its ceiling"
DEFAULT_WHY = r"with no row in tools/comment-ceilings.txt is above the default ceiling"
COMPLETION = re.compile(
    r"^check:comments: (\d+) files, (\d+) added comment lines, (\d+) ledger rows; clean(?:;[^\n]*)?$",
    re.M,
)


def pad(n, start=0):
    return "".join("var v%d = %d\n" % (i, i) for i in range(start, start + n))


def go(body_comments=(), code=10, pkg="foo"):
    """A Go file: package line, then comment lines, then `code` code lines.
    Non-blank lines = 1 + len(comments) + code."""
    s = "package %s\n\n" % pkg
    s += "".join(c + "\n" for c in body_comments)
    s += pad(code)
    return s


def shared(comments, nonblank, pkg="foo"):
    """A Go file whose share is comments/nonblank*100: `comments` `//` lines,
    each its own block, among `nonblank` non-blank lines."""
    code = nonblank - 1 - comments
    assert 0 <= comments <= code, (comments, nonblank)
    body = []
    for i in range(comments):
        body.append("// Keep item %d in order." % i)
        body.append("var c%d = %d" % (i, i))
    return go(body, code=code - comments, pkg=pkg)


class Repo:
    def __init__(self):
        self.dir = tempfile.mkdtemp(prefix="ccqa-")
        os.makedirs(os.path.join(self.dir, "tools"))
        for f in ("check-comments.py", "check-new-prose.py"):
            shutil.copy(os.path.join(HERE, f), os.path.join(self.dir, "tools", f))
        self.git("init", "-q", "-b", "main")

    def git(self, *args):
        return subprocess.run(
            ["git", "-c", "user.email=qa@x", "-c", "user.name=qa", *args],
            cwd=self.dir, capture_output=True, text=True, check=True,
        )

    def write(self, path, text):
        p = os.path.join(self.dir, path)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w") as fh:
            fh.write(text)

    def read(self, path):
        with open(os.path.join(self.dir, path)) as fh:
            return fh.read()

    def rm(self, path):
        os.remove(os.path.join(self.dir, path))

    def ledger(self, rows):
        self.write(
            "tools/comment-ceilings.txt",
            "# ledger\n" + "".join("%s  %s\n" % (p, s) for p, s in rows.items()),
        )

    def register(self, tag):
        """docs/requirements.md with a `project: <tag>` line, or with no
        `project:` line at all when tag is ""."""
        line = "project: %s\n\n" % tag if tag else ""
        self.write(
            "docs/requirements.md",
            "# Requirements\n\n" + line + "| Id | Requirement | Verified by |\n|---|---|---|\n",
        )

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-qm", "base", "--allow-empty")

    def run(self, *args):
        r = subprocess.run(
            [sys.executable, GATE, *args], cwd=self.dir, capture_output=True, text=True
        )
        return r.returncode, r.stdout + r.stderr

    def close(self):
        shutil.rmtree(self.dir, ignore_errors=True)


class QA(unittest.TestCase):
    def setUp(self):
        self.r = Repo()

    def tearDown(self):
        self.r.close()

    def base(self, files, rows=None, register=None):
        for p, t in files.items():
            self.r.write(p, t)
        self.r.ledger(rows or {})
        if register is not None:
            self.r.register(register)
        self.r.commit()

    def gate(self):
        return self.r.run("main")

    def assertRefused(self, res, msg="", why=None):
        code, out = res
        self.assertEqual(code, 1, msg + "\n" + out)
        if why is not None:
            self.assertRegex(out, why)
        self.assertIsNone(COMPLETION.search(out), out)

    def assertClean(self, res):
        code, out = res
        self.assertEqual(code, 0, out)
        self.assertRegex(out, COMPLETION)

    # ---------------------------------------------------------------- VTT-050

    BANNED = [
        "// Keep the order; 2026-09-01 decided it.",
        "// Keep the order, measured on the table.",
        "// Keep the order; it used to be reversed.",
        "// Keep the order; previously reversed.",
        "// Keep the order; an earlier version reversed it.",
        "// Keep the order; it turned out reversed breaks.",
        "// Keep the order; review found it reversed.",
        "// Keep the order, spec §4.",
        "// Keep the order, task 3.",
        "// Keep the order, see #123.",
    ]

    # VTT-050
    def test_qa_each_banned_term_on_an_added_go_line_is_refused(self):
        self.base({"internal/foo/a.go": go(code=40)})
        for line in self.BANNED:
            with self.subTest(line=line):
                self.r.write("internal/foo/a.go", go([line], code=40))
                self.assertRefused(self.gate(), line, why=BANNED_WHY)

    # VTT-050
    def test_qa_banned_terms_are_case_insensitive(self):
        self.base({"internal/foo/a.go": go(code=40)})
        for line in ["// Keep it. MEASURED.", "// Keep it. Turned Out.", "// Keep it, TASK 3."]:
            with self.subTest(line=line):
                self.r.write("internal/foo/a.go", go([line], code=40))
                self.assertRefused(self.gate(), line, why=BANNED_WHY)

    # VTT-050
    def test_qa_banned_term_in_typescript_star_and_slash_star_lines_is_refused(self):
        ts_base = "export const a = 1;\n" * 40
        self.base({"client/src/a.ts": ts_base})
        for block in [
            "// Keep it; previously broken.\n",
            "/**\n * Keep it; previously broken.\n */\n",
            "/* Keep it; previously broken. */\n",
        ]:
            with self.subTest(block=block):
                self.r.write("client/src/a.ts", block + ts_base)
                self.assertRefused(self.gate(), block, why=BANNED_WHY)

    # VTT-050
    def test_qa_banned_term_in_cmd_is_refused(self):
        self.base({"cmd/vtt/a.go": go(code=40, pkg="main")})
        self.r.write("cmd/vtt/a.go", go(["// Keep it; previously broken."], code=40, pkg="main"))
        self.assertRefused(self.gate(), why=BANNED_WHY)

    # VTT-050
    def test_qa_a_warning_comment_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["// Do not reorder these; the fold depends on it."], code=40))
        self.assertClean(self.gate())

    # VTT-050
    def test_qa_a_date_inside_a_docs_path_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["// See docs/reports/2026-09-01-the-fold.md."], code=40))
        self.assertClean(self.gate())

    # VTT-050
    def test_qa_a_date_outside_the_docs_path_on_the_same_line_is_refused(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go",
                     go(["// See docs/reports/2026-09-01-x.md, decided 2026-09-02."], code=40))
        self.assertRefused(self.gate(), why=BANNED_WHY)

    # VTT-050 (SPEC-010 "What is outside": a VTT-NNN citation is a pointer)
    def test_qa_a_requirement_id_pointer_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["// Holds VTT-050 and SPEC-010."], code=40))
        self.assertClean(self.gate())

    # VTT-050 (SPEC-010 "What is outside": a //go: or //nolint directive is not refused)
    def test_qa_a_go_directive_carrying_a_banned_term_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["//go:generate stringer -type=Task -trimprefix previously"], code=40))
        self.assertClean(self.gate())

    # SPEC-010 "What is outside": the directive part is not read; a trailing
    # // reason is prose and is.
    # VTT-050
    def test_qa_a_nolint_directive_with_a_banned_reason_is_refused(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["//nolint:errcheck // measured"], code=40))
        self.assertRefused(self.gate(), why=r"carries 'measured'")

    # VTT-050 (SPEC-010 "What is outside": plain directives pass)
    def test_qa_plain_directives_pass(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["//go:noinline", "func f() {}", "//nolint:errcheck"], code=40))
        self.assertClean(self.gate())

    # VTT-050 (SPEC-010: an [anchor:...] marker is allowed as a directive is)
    def test_qa_an_anchor_marker_carrying_a_banned_word_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["// [anchor:measured-share-guard]"], code=40))
        self.assertClean(self.gate())

    # VTT-050 (only ADDED lines are read)
    def test_qa_a_banned_line_already_in_the_base_is_not_refused(self):
        self.base({"internal/foo/a.go": go(["// It used to be reversed."], code=40)})
        self.r.write("internal/foo/a.go", go(["// It used to be reversed."], code=41))
        self.assertClean(self.gate())

    # VTT-050 (scope: outside internal/, cmd/, client/src)
    def test_qa_banned_line_outside_the_roots_is_not_read(self):
        self.base({"internal/foo/a.go": go(code=40), "pkg/b.go": go(code=4)})
        self.r.write("pkg/b.go", go(["// previously broken"], code=4))
        self.r.write("client/b.ts", "// previously broken\nexport const a = 1;\n")
        self.assertClean(self.gate())

    # VTT-050 (scope: generated files are out)
    def test_qa_banned_line_in_generated_files_is_not_read(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/x.pb.go", go(["// previously broken"], code=2))
        self.r.write("client/src/x.d.ts", "// previously broken\nexport declare const a: number;\n")
        self.r.write("client/src/gen/y.ts", "// previously broken\nexport const a = 1;\n")
        self.assertClean(self.gate())

    # SPEC-010 defers what a comment line is to the gate, whose usage text
    # counts every line of a /* ... */ block in Go as well as TypeScript.
    # VTT-050
    def test_qa_a_go_slash_star_block_is_read_as_comment(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["/* previously broken */"], code=40))
        self.assertRefused(self.gate(), why=r"carries 'previously'")

    # ---------------------------------------------------------------- VTT-052

    def block(self, n):
        return ["// Keep line %d in order." % i for i in range(n)]

    # VTT-052
    def test_qa_a_block_over_the_bound_that_the_change_adds_to_is_refused(self):
        self.base({"internal/foo/a.go": go(self.block(BOUND), code=40)})
        self.r.write("internal/foo/a.go", go(self.block(BOUND + 1), code=40))
        self.assertRefused(self.gate(), why=BLOCK_WHY)

    # VTT-052
    def test_qa_a_block_at_exactly_the_bound_passes(self):
        self.base({"internal/foo/a.go": go(self.block(BOUND - 1), code=40)})
        self.r.write("internal/foo/a.go", go(self.block(BOUND), code=40))
        self.assertClean(self.gate())

    # VTT-052
    def test_qa_a_long_block_the_change_does_not_touch_passes(self):
        long = self.block(BOUND + 4)
        self.base({"internal/foo/a.go": go(long, code=40)},
                  {"internal/foo/a.go": "19.6"})  # 10 of 51
        self.r.write("internal/foo/a.go", go(long, code=41))
        self.assertClean(self.gate())

    # VTT-052
    def test_qa_a_blank_line_does_not_end_a_block(self):
        self.base({"internal/foo/a.go": go(code=40)})
        lines = self.block(3) + [""] + self.block(4)
        self.r.write("internal/foo/a.go", go(lines, code=40))
        self.assertRefused(self.gate(), why=BLOCK_WHY)

    # VTT-052
    def test_qa_a_code_line_ends_a_block(self):
        self.base({"internal/foo/a.go": go(code=40)})
        lines = self.block(4) + ["var split = 0"] + self.block(4)
        self.r.write("internal/foo/a.go", go(lines, code=40))
        self.assertClean(self.gate())

    # VTT-052
    def test_qa_a_long_go_package_doc_is_excepted(self):
        self.base({"internal/foo/a.go": go(code=40)})
        doc = "".join("// Package foo keeps line %d.\n" % i for i in range(BOUND + 4))
        self.r.write("internal/foo/a.go", doc + go(code=40))
        self.assertClean(self.gate())

    # VTT-052
    def test_qa_a_long_typescript_block_is_refused(self):
        ts = "export const a = 1;\n" * 40
        self.base({"client/src/a.ts": ts})
        blk = "/**\n" + "".join(" * Keep line %d.\n" % i for i in range(BOUND)) + " */\n"
        self.r.write("client/src/a.ts", blk + ts)
        self.assertRefused(self.gate(), why=BLOCK_WHY)

    # ---------------------------------------------------------------- VTT-053

    # VTT-053
    def test_qa_adding_a_comment_line_that_lifts_a_file_over_its_ceiling_is_refused(self):
        # 2 comments of 10 non-blank lines = 20.0
        self.base({"internal/foo/a.go": go(["// Keep a.", "var s = 0", "// Keep b."], code=6)},
                  {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go",
                     go(["// Keep a.", "var s = 0", "// Keep b.", "var t = 0", "// Keep c."], code=6))
        self.assertRefused(self.gate(), why=CEIL_WHY)  # 3 of 12 = 25.0

    # VTT-053
    def test_qa_adding_a_comment_line_that_stays_at_the_ceiling_passes(self):
        self.base({"internal/foo/a.go": go(["// Keep a.", "var s = 0", "// Keep b."], code=6)},
                  {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go",
                     go(["// Keep a.", "var s = 0", "// Keep b.", "var t = 0", "// Keep c."], code=9))
        self.assertClean(self.gate())  # 3 of 15 = 20.0

    # VTT-053 (SPEC-010 Consequences: removed code alone is a notice)
    def test_qa_a_rise_caused_only_by_removed_code_is_a_notice(self):
        self.base({"internal/foo/a.go": go(["// Keep a.", "var s = 0", "// Keep b."], code=6)},
                  {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go", go(["// Keep a.", "var s = 0", "// Keep b."], code=4))
        code, out = self.gate()  # 2 of 8 = 25.0
        self.assertEqual(code, 0, out)
        self.assertRegex(out, COMPLETION)
        self.assertRegex(out, r"notice: internal/foo/a\.go .*above its ceiling")  # "is reported"

    # ---------------------------------------------------------------- VTT-054

    def twenty(self):
        return go(["// Keep a.", "var s = 0", "// Keep b."], code=6)  # 20.0

    # VTT-054
    def test_qa_a_row_raised_above_the_base_is_refused(self):
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "20.0"})
        self.r.ledger({"internal/foo/a.go": "21.0"})
        self.assertRefused(self.gate(), why=RAISE_WHY)

    # VTT-054
    def test_qa_a_hand_lowered_row_is_not_refused(self):
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "20.5"})
        self.r.ledger({"internal/foo/a.go": "20.0"})
        self.assertClean(self.gate())

    # VTT-054
    def test_qa_a_new_row_above_default_is_refused(self):
        self.base({"internal/foo/a.go": go(code=40)})
        # 3 of 11 = 27.3, new file with a row the base does not hold, row at its share
        self.r.write("internal/foo/b.go", go(["// A.", "var s = 0", "// B.", "var t = 0", "// C."], code=5))
        self.r.ledger({"internal/foo/b.go": "27.3"})
        self.assertRefused(self.gate(), why=NEWROW_WHY)

    # VTT-054
    def test_qa_a_new_row_at_or_under_default_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/b.go", self.twenty())
        self.r.ledger({"internal/foo/b.go": "20.0"})
        self.assertClean(self.gate())

    def forty(self):
        # 4 of 10 = 40.0, comments separated so no block over the bound
        return go(["// A.", "var s = 0", "// B.", "var t = 0", "// C.", "var u = 0", "// D."], code=2)

    # VTT-054 (renames followed)
    def test_qa_a_renamed_file_keeps_its_row_above_default(self):
        self.base({"internal/foo/a.go": self.forty()}, {"internal/foo/a.go": "40.0"})
        self.r.git("mv", "internal/foo/a.go", "internal/foo/b.go")
        self.r.ledger({"internal/foo/b.go": "40.0"})
        self.assertClean(self.gate())

    # VTT-054 (renames followed)
    def test_qa_a_renamed_file_raised_above_its_old_row_is_refused(self):
        self.base({"internal/foo/a.go": self.forty()}, {"internal/foo/a.go": "40.0"})
        self.r.git("mv", "internal/foo/a.go", "internal/foo/b.go")
        self.r.ledger({"internal/foo/b.go": "41.0"})
        self.assertRefused(self.gate(), why=RAISE_WHY)

    # VTT-054 (SPEC-010: base with no ledger skips the raise check and says so)
    def test_qa_a_base_with_no_ledger_skips_the_raise_check_and_says_so(self):
        self.r.write("internal/foo/a.go", self.forty())
        self.r.commit()
        self.r.ledger({"internal/foo/a.go": "40.0"})
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        self.assertRegex(out, COMPLETION)
        self.assertRegex(out.lower(), r"skip")

    # VTT-054
    def test_qa_write_ledger_never_raises_a_row(self):
        # b.go falls from 40.0 to 20.0 in the same run, so the run is shown to write
        self.base({"internal/foo/a.go": self.twenty(), "internal/foo/b.go": self.forty()},
                  {"internal/foo/a.go": "20.0", "internal/foo/b.go": "40.0"})
        self.r.write("internal/foo/a.go", self.forty())
        self.r.write("internal/foo/b.go", self.twenty())
        code, out = self.r.run("--write-ledger")
        self.assertEqual(code, 0, out)
        led = self.r.read("tools/comment-ceilings.txt")
        a = re.search(r"^internal/foo/a\.go\s+(\S+)$", led, re.M)
        b = re.search(r"^internal/foo/b\.go\s+(\S+)$", led, re.M)
        self.assertIsNotNone(a, led)
        self.assertIsNotNone(b, led)
        self.assertLessEqual(float(a.group(1)), 20.0)
        self.assertAlmostEqual(float(b.group(1)), 20.0, places=1)

    # VTT-055
    def test_qa_write_ledger_lowers_a_row_to_the_measured_share(self):
        self.base({"internal/foo/a.go": self.forty()}, {"internal/foo/a.go": "40.0"})
        self.r.write("internal/foo/a.go", self.twenty())
        code, out = self.r.run("--write-ledger")
        self.assertEqual(code, 0, out)
        m = re.search(r"^internal/foo/a\.go\s+(\S+)$", self.r.read("tools/comment-ceilings.txt"), re.M)
        self.assertIsNotNone(m)
        self.assertAlmostEqual(float(m.group(1)), 20.0, places=1)
        self.assertClean(self.gate())

    # VTT-056
    def test_qa_write_ledger_drops_a_row_whose_file_is_gone(self):
        self.base({"internal/foo/a.go": self.twenty(), "internal/foo/b.go": self.twenty()},
                  {"internal/foo/a.go": "20.0", "internal/foo/b.go": "20.0"})
        self.r.rm("internal/foo/b.go")
        code, out = self.r.run("--write-ledger")
        self.assertEqual(code, 0, out)
        led = self.r.read("tools/comment-ceilings.txt")
        self.assertNotIn("internal/foo/b.go", led)
        self.assertIn("internal/foo/a.go", led)
        self.assertClean(self.gate())

    # ---------------------------------------------------------------- VTT-055

    # VTT-055
    def test_qa_a_share_more_than_the_band_under_its_ceiling_is_refused(self):
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "21.1"})
        self.assertRefused(self.gate(), why=BAND_WHY)

    # VTT-055
    def test_qa_a_share_exactly_the_band_under_its_ceiling_passes(self):
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "21.0"})
        self.assertClean(self.gate())

    # ---------------------------------------------------------------- VTT-056

    # VTT-056
    def test_qa_a_row_naming_a_missing_file_is_refused(self):
        self.base({"internal/foo/a.go": self.twenty()},
                  {"internal/foo/a.go": "20.0", "internal/foo/gone.go": "10.0"})
        self.assertRefused(self.gate(), why=STALE_WHY)

    # ---------------------------------------------------------------- VTT-057

    # VTT-057
    def test_qa_a_file_with_no_row_above_default_is_refused(self):
        self.base({"internal/foo/a.go": go(code=40)})
        # 3 of 11 = 27.3
        self.r.write("internal/foo/b.go", go(["// A.", "var s = 0", "// B.", "var t = 0", "// C."], code=5))
        self.assertRefused(self.gate(), why=DEFAULT_WHY)

    # VTT-057
    def test_qa_a_file_with_no_row_at_default_passes(self):
        self.base({"internal/foo/a.go": go(code=40)})
        # 3 of 12 = 25.0
        self.r.write("internal/foo/b.go", go(["// A.", "var s = 0", "// B.", "var t = 0", "// C."], code=6))
        self.assertClean(self.gate())

    # VTT-057 (without an added comment line: the refusal list names it unconditionally)
    def test_qa_a_base_file_with_no_row_above_default_is_refused_without_a_change(self):
        self.base({"internal/foo/a.go": self.forty()})
        self.assertRefused(self.gate(), why=DEFAULT_WHY)

    # ---------------------------------------------------------------- VTT-058

    # VTT-058
    def test_qa_completion_line_names_files_added_comment_lines_and_rows(self):
        self.base({"internal/foo/a.go": self.twenty(), "internal/foo/b.go": go(code=40)},
                  {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/b.go", go(["// Keep x.", "var s = 0", "// Keep y."], code=40))
        self.r.write("client/src/c.ts", "// Keep z.\n" + "export const a = 1;\n" * 10)
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        m = COMPLETION.search(out)
        self.assertIsNotNone(m, out)
        self.assertEqual(m.groups(), ("3", "3", "1"), out)

    # VTT-058
    def test_qa_no_file_in_scope_exits_2(self):
        self.base({"pkg/a.go": go(code=4)})
        code, out = self.gate()
        self.assertEqual(code, 2, out)
        self.assertIsNone(COMPLETION.search(out))

    # VTT-058
    def test_qa_an_unknown_base_ref_exits_2(self):
        self.base({"internal/foo/a.go": go(code=40)})
        code, out = self.r.run("no-such-ref")
        self.assertEqual(code, 2, out)
        self.assertIsNone(COMPLETION.search(out))

    # VTT-058
    def test_qa_outside_a_git_repository_exits_2(self):
        self.base({"internal/foo/a.go": go(code=40)})
        shutil.rmtree(os.path.join(self.r.dir, ".git"))
        code, out = self.gate()
        self.assertEqual(code, 2, out)

    # VTT-058
    def test_qa_a_missing_ledger_exits_2(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.rm("tools/comment-ceilings.txt")
        code, out = self.gate()
        self.assertEqual(code, 2, out)
        self.assertIsNone(COMPLETION.search(out))

    # VTT-058
    def test_qa_no_arguments_exits_2(self):
        code, out = self.r.run()
        self.assertEqual(code, 2, out)

    # ---------------------------------------------------------------- VTT-059
    #
    # SPEC-010: "A `//` line carrying only requirement ids of the register's
    # tag, spaces between, is a citation line: it counts in no comment share,
    # is not an added comment line and has no place in a block, so adding one
    # moves nothing; a line inside a `/* ... */` block is a block line
    # whatever it carries".
    # The gate's usage text gives the shapes `// VTT-042` and
    # `// VTT-048 VTT-049`, takes the tag from the register's `project:` line,
    # sets a citation line aside "as a blank line does not", and with no
    # register or no `project:` line sets no line aside "and the completion
    # line says so". Every register these tests write holds the tag line and
    # an empty table: the record reads the tag, not the rows.

    def added(self, res):
        """The completion line's added-comment-line figure of a clean run."""
        code, out = res
        self.assertEqual(code, 0, out)
        m = COMPLETION.search(out)
        self.assertIsNotNone(m, out)
        return int(m.group(2))

    def cited(self, *lines):
        """twenty() with the given lines after its last comment: 2 of 10 plus
        whatever the gate counts of `lines`."""
        return go(["// Keep a.", "var s = 0", "// Keep b."] + list(lines), code=6)

    # VTT-059
    def test_qa_a_line_carrying_only_ids_of_the_registers_tag_is_not_an_added_comment_line(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        for line in ["// VTT-042", "// VTT-048 VTT-049", "\t// VTT-042"]:
            with self.subTest(line=line):
                self.r.write("internal/foo/a.go", go([line], code=40))
                self.assertEqual(self.added(self.gate()), 0, line)

    # VTT-059
    def test_qa_a_word_beside_the_id_makes_an_added_comment_line(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        for line in ["// VTT-042 holds this.", "// Holds VTT-042", "// See VTT-048 VTT-049"]:
            with self.subTest(line=line):
                self.r.write("internal/foo/a.go", go([line], code=40))
                self.assertEqual(self.added(self.gate()), 1, line)

    # "carrying only requirement ids": a comma is not an id, and the usage
    # text's two-id shape is space-separated.
    # VTT-059
    def test_qa_a_comma_between_two_ids_makes_an_added_comment_line(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        self.r.write("internal/foo/a.go", go(["// VTT-048, VTT-049"], code=40))
        self.assertEqual(self.added(self.gate()), 1)

    # VTT-059
    def test_qa_the_tag_is_the_registers_project_line_and_another_tag_counts(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="ABC")
        self.r.write("internal/foo/a.go", go(["// ABC-001"], code=40))
        self.assertEqual(self.added(self.gate()), 0, "the register's own tag")
        self.r.write("internal/foo/a.go", go(["// VTT-042"], code=40))
        self.assertEqual(self.added(self.gate()), 1, "a tag the register does not declare")

    # VTT-059
    def test_qa_a_typescript_slash_line_carrying_only_ids_is_a_citation_line(self):
        ts = "export const a = 1;\n" * 40
        self.base({"client/src/a.ts": ts}, register="VTT")
        self.r.write("client/src/a.ts", "// VTT-042\n" + ts)
        self.assertEqual(self.added(self.gate()), 0)

    # A line of a /* */ block is a block line in either language, whatever it
    # carries (SPEC-010).
    # VTT-059
    def test_qa_a_go_slash_star_line_carrying_only_ids_is_a_comment_line(self):
        # Ruled at adjudication: only a `//` line is a citation line; a /* */
        # line is a block line whatever it carries.
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        self.r.write("internal/foo/a.go", go(["/* VTT-042 */"], code=40))
        self.assertEqual(self.added(self.gate()), 1)

    # VTT-059
    def test_qa_a_star_line_inside_a_typescript_block_carrying_only_ids_is_a_block_line(self):
        # Ruled at adjudication, as the Go case above.
        ts = "export const a = 1;\n" * 40
        self.base({"client/src/a.ts": ts}, register="VTT")
        self.r.write("client/src/a.ts", "/**\n * Keep it.\n * VTT-042\n */\n" + ts)
        self.assertEqual(self.added(self.gate()), 4)  # /**, * Keep it., * VTT-042, */

    # VTT-059
    def test_qa_a_file_at_its_ceiling_that_gains_a_citation_line_stays_clean(self):
        # counted as comment: 3 of 11 = 27.3, refused; counted as a non-blank
        # line only: 2 of 11 = 18.2, more than the band under; counted as
        # neither: 2 of 10 = 20.0
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "20.0"}, register="VTT")
        self.r.write("internal/foo/a.go", self.cited("// VTT-042"))
        self.assertClean(self.gate())

    # "as a blank line does not": the line is not among the non-blank lines
    # the share divides by either.
    # VTT-059
    def test_qa_a_citation_line_is_not_a_non_blank_line_of_the_share(self):
        # exactly the band under its ceiling before; 2 of 12 = 16.7 would be over it
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "21.0"}, register="VTT")
        self.r.write("internal/foo/a.go", self.cited("// VTT-042", "// VTT-048 VTT-049"))
        self.assertClean(self.gate())

    # VTT-059
    def test_qa_a_file_above_its_ceiling_that_gains_only_a_citation_line_is_a_notice(self):
        self.base({"internal/foo/a.go": self.forty()}, {"internal/foo/a.go": "30.0"}, register="VTT")
        self.r.write("internal/foo/a.go",
                     go(["// A.", "var s = 0", "// B.", "var t = 0", "// C.", "var u = 0", "// D.", "// VTT-042"], code=2))
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        self.assertRegex(out, COMPLETION)
        self.assertRegex(out, r"notice: internal/foo/a\.go .*above its ceiling")

    # VTT-059
    def test_qa_a_block_at_the_bound_plus_a_citation_line_passes(self):
        self.base({"internal/foo/a.go": go(self.block(BOUND - 1), code=40)}, register="VTT")
        for where, lines in [("before", ["// VTT-042"] + self.block(BOUND)),
                             ("after", self.block(BOUND) + ["// VTT-042"])]:
            with self.subTest(where=where):
                self.r.write("internal/foo/a.go", go(lines, code=40))
                self.assertClean(self.gate())

    # VTT-059
    def test_qa_a_citation_line_between_two_halves_of_a_long_block_does_not_split_it(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        b = self.block(BOUND + 2)
        self.r.write("internal/foo/a.go", go(b[:4] + ["// VTT-042"] + b[4:], code=40))
        self.assertRefused(self.gate(), why=r"comment block of %d lines is over the bound" % (BOUND + 2))

    # "has no place in a block, so adding one moves nothing": a long block the
    # change adds only a citation line to was not added to.
    # VTT-059
    def test_qa_a_citation_line_added_to_a_long_block_does_not_add_a_line_to_it(self):
        long = self.block(BOUND + 4)  # 10 of 51 = 19.6, under the default with no row
        self.base({"internal/foo/a.go": go(long, code=40)}, register="VTT")
        self.r.write("internal/foo/a.go", go(long + ["// VTT-042"], code=40))
        self.assertClean(self.gate())

    # VTT-059
    def test_qa_the_completion_line_counts_added_comment_lines_without_citation_lines(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="VTT")
        self.r.write("internal/foo/a.go",
                     go(["// Keep x.", "// VTT-042", "var s = 0", "// Keep y.", "// VTT-048 VTT-049"], code=40))
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        m = COMPLETION.search(out)
        self.assertIsNotNone(m, out)
        self.assertEqual(m.group(2), "2", out)
        self.assertRegex(m.group(0), r"; clean$")  # nothing to say: a register was read

    # VTT-059
    def test_qa_write_ledger_writes_the_share_without_citation_lines(self):
        # as comment lines: 4 of 12 = 33.3; as non-blank only: 2 of 12 = 16.7
        self.base({"internal/foo/a.go": self.forty()}, {"internal/foo/a.go": "40.0"}, register="VTT")
        self.r.write("internal/foo/a.go", self.cited("// VTT-042", "// VTT-048 VTT-049"))
        code, out = self.r.run("--write-ledger")
        self.assertEqual(code, 0, out)
        led = self.r.read("tools/comment-ceilings.txt")
        m = re.search(r"^internal/foo/a\.go\s+(\S+)$", led, re.M)
        self.assertIsNotNone(m, led)
        self.assertEqual(m.group(1), "20.0", led)
        self.assertClean(self.gate())

    # VTT-059
    def test_qa_with_no_register_a_line_carrying_only_ids_is_an_added_comment_line(self):
        self.base({"internal/foo/a.go": self.twenty()}, {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go", self.cited("// VTT-042"))
        self.assertRefused(self.gate(), why=CEIL_WHY)  # 3 of 11 = 27.3

    # VTT-059
    def test_qa_with_no_register_the_completion_line_says_no_line_was_set_aside(self):
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/a.go", go(["// VTT-042"], code=40))
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        m = COMPLETION.search(out)
        self.assertIsNotNone(m, out)
        self.assertEqual(m.group(2), "1", out)
        self.assertNotRegex(m.group(0), r"; clean$")
        self.assertRegex(m.group(0), r"(?i)register|set aside|citation")

    # VTT-059
    def test_qa_with_no_project_line_every_slash_line_counts_and_the_completion_line_says_so(self):
        self.base({"internal/foo/a.go": go(code=40)}, register="")
        self.r.write("internal/foo/a.go", go(["// VTT-042"], code=40))
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        m = COMPLETION.search(out)
        self.assertIsNotNone(m, out)
        self.assertEqual(m.group(2), "1", out)
        self.assertNotRegex(m.group(0), r"; clean$")
        self.assertRegex(m.group(0), r"(?i)project|register|set aside|citation")

    # ---------------------------------------------------------------- VTT-078
    #
    # VTT-078: "A finding or notice that compares a share with a ceiling
    # prints the share on the side of that ceiling its verdict names." The
    # gate's usage text: a share is "compared unrounded; a ledger row holds a
    # share as a tenth, rounded up; a finding or notice prints the share it
    # compared, rounded to the fewest decimals, two at least, that keep it on
    # the side of the compared value the verdict names". The band is 1.0 and
    # the default ceiling 25.0 as the usage text names them.
    # Each near-edge fixture below is a ratio c/n whose share sits within a
    # twentieth of the value it is compared with, where a tenth reads AT that
    # value, or within a two-hundredth, where two decimals do; n is chosen so
    # that rounding and truncation give the same decimal count, except 11 of
    # 57, where truncation would stop at two. The side is checked with
    # Fraction on the printed strings, never with floats.

    def finding(self, out, path):
        """The one line of `out` that names `path` against a ceiling."""
        lines = [l for l in out.splitlines() if path in l and "ceiling" in l]
        self.assertEqual(len(lines), 1, out)
        return lines[0]

    def printed(self, line):
        """(share, ceiling, band) as the line prints them: the ceiling is the
        number after `ceiling`, the band the number after `more than`, and
        the share the one number left, wherever it stands."""
        ceiling = re.search(r"ceiling (\d+\.\d+)", line)
        band = re.search(r"more than (\d+\.\d+) under", line)
        rest = re.findall(r"(?<!ceiling )(?<!than )\b\d+\.\d+\b", line)
        self.assertEqual(len(rest), 1, line)
        return rest[0], ceiling and ceiling.group(1), band and band.group(1)

    @staticmethod
    def decimals(s):
        return len(s.split(".")[1])

    # "compared unrounded": 81 of 404 = 20.0495 is above 20.0, though a tenth
    # of it reads 20.0.
    # VTT-078
    def test_qa_a_ceiling_refusal_within_a_twentieth_prints_the_share_above_the_ceiling(self):
        self.base({"internal/foo/a.go": shared(80, 403)}, {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go", shared(81, 404))
        res = self.gate()
        self.assertRefused(res, why=CEIL_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/a.go"))
        self.assertEqual(self.decimals(share), 2, share)
        self.assertGreater(Fraction(share), Fraction("20.0"), share)

    # VTT-078
    def test_qa_a_band_refusal_within_a_twentieth_of_the_edge_prints_the_share_past_the_band(self):
        # 61 of 304 = 20.0658 with ceiling 21.1: 1.034 under; a tenth reads 20.1, exactly the band
        self.base({"internal/foo/a.go": shared(61, 304)}, {"internal/foo/a.go": "21.1"})
        res = self.gate()
        self.assertRefused(res, why=BAND_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/a.go"))
        self.assertEqual(self.decimals(share), 2, share)
        self.assertGreater(Fraction("21.1") - Fraction(share), Fraction(str(BAND)), share)

    # VTT-078
    def test_qa_a_default_refusal_within_a_twentieth_prints_the_share_above_the_default(self):
        # 151 of 603 = 25.0415 with no row; a tenth reads 25.0
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/b.go", shared(151, 603))
        res = self.gate()
        self.assertRefused(res, why=DEFAULT_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/b.go"))
        self.assertEqual(self.decimals(share), 2, share)
        self.assertGreater(Fraction(share), Fraction(str(DEFAULT)), share)

    # SPEC-010: a file above its ceiling with no comment line added "is
    # reported and not refused"; the usage text: "a finding or notice prints
    # the share it compared".
    # VTT-078
    def test_qa_a_notice_prints_the_share_above_the_ceiling(self):
        # 100 of 503 = 19.88 under 20.0; four code lines leave: 100 of 499 = 20.04
        self.base({"internal/foo/a.go": shared(100, 503)}, {"internal/foo/a.go": "20.0"})
        self.r.write("internal/foo/a.go", shared(100, 499))
        code, out = self.gate()
        self.assertEqual(code, 0, out)
        self.assertRegex(out, COMPLETION)
        line = self.finding(out, "internal/foo/a.go")
        self.assertRegex(line, r"notice: internal/foo/a\.go .*above its ceiling")
        share, _, _ = self.printed(line)
        self.assertEqual(self.decimals(share), 2, share)
        self.assertGreater(Fraction(share), Fraction("20.0"), share)

    # "the fewest decimals ... that keep it on the side": two do not, here.
    # VTT-078
    def test_qa_a_ceiling_refusal_within_a_two_hundredth_prints_a_third_decimal(self):
        # 21 of 83 = 25.3012 above 25.3; "25.30" reads at it
        self.base({"internal/foo/a.go": shared(20, 82)}, {"internal/foo/a.go": "25.3"})
        self.r.write("internal/foo/a.go", shared(21, 83))
        res = self.gate()
        self.assertRefused(res, why=CEIL_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/a.go"))
        self.assertEqual(self.decimals(share), 3, share)
        self.assertGreater(Fraction(share), Fraction("25.3"), share)

    # VTT-078
    def test_qa_a_band_refusal_within_a_two_hundredth_of_the_edge_prints_a_third_decimal(self):
        # 11 of 57 = 19.29825 with ceiling 20.3: 1.0018 under; "19.30" reads exactly the band under
        self.base({"internal/foo/a.go": shared(11, 57)}, {"internal/foo/a.go": "20.3"})
        res = self.gate()
        self.assertRefused(res, why=BAND_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/a.go"))
        self.assertEqual(self.decimals(share), 3, share)
        self.assertGreater(Fraction("20.3") - Fraction(share), Fraction(str(BAND)), share)

    # VTT-078
    def test_qa_a_default_refusal_within_a_two_hundredth_prints_a_third_decimal(self):
        # 1251 of 5003 = 25.004997 with no row; "25.00" reads at the default
        self.base({"internal/foo/a.go": go(code=40)})
        self.r.write("internal/foo/b.go", shared(1251, 5003))
        res = self.gate()
        self.assertRefused(res, why=DEFAULT_WHY)
        share, _, _ = self.printed(self.finding(res[1], "internal/foo/b.go"))
        self.assertEqual(self.decimals(share), 3, share)
        self.assertGreater(Fraction(share), Fraction(str(DEFAULT)), share)

    # "two at least": a share a tenth would separate is still printed to two.
    # VTT-078
    def test_qa_a_finding_prints_two_decimals_even_where_one_would_separate(self):
        # a.go: 4 of 10 = 40.0 above 20.0 with comment lines added; b.go: 2 of 10 = 20.0, 1.1 under 21.1
        self.base({"internal/foo/a.go": self.twenty(), "internal/foo/b.go": self.twenty()},
                  {"internal/foo/a.go": "20.0", "internal/foo/b.go": "21.1"})
        self.r.write("internal/foo/a.go", self.forty())
        res = self.gate()
        self.assertRefused(res, why=CEIL_WHY)
        self.assertRegex(res[1], BAND_WHY)
        a, _, _ = self.printed(self.finding(res[1], "internal/foo/a.go"))
        b, _, _ = self.printed(self.finding(res[1], "internal/foo/b.go"))
        self.assertEqual(a, "40.00", res[1])
        self.assertEqual(b, "20.00", res[1])

    # The ceiling a finding names is the ledger row, "a tenth"; the band and
    # the default are "1.0 point" and "25.0" as the usage text spells them.
    # VTT-078
    def test_qa_the_ceiling_the_band_and_the_default_in_a_finding_read_as_tenths(self):
        self.base({"internal/foo/a.go": shared(80, 403), "internal/foo/b.go": shared(61, 304)},
                  {"internal/foo/a.go": "20.0", "internal/foo/b.go": "21.1"})
        self.r.write("internal/foo/a.go", shared(81, 404))
        self.r.write("internal/foo/c.go", shared(151, 603))
        res = self.gate()
        self.assertRefused(res, why=CEIL_WHY)
        _, a_ceiling, a_band = self.printed(self.finding(res[1], "internal/foo/a.go"))
        _, b_ceiling, b_band = self.printed(self.finding(res[1], "internal/foo/b.go"))
        _, c_ceiling, c_band = self.printed(self.finding(res[1], "internal/foo/c.go"))
        self.assertEqual(a_ceiling, "20.0", res[1])
        self.assertIsNone(a_band, res[1])
        self.assertEqual((b_ceiling, b_band), ("21.1", "1.0"), res[1])
        self.assertEqual(c_ceiling, "25.0", res[1])
        self.assertIsNone(c_band, res[1])

    # "a ledger row holds a share as a tenth, rounded up": the decimals a
    # finding prints do not reach the ledger.
    # VTT-078
    def test_qa_write_ledger_writes_a_tenth_rounded_up(self):
        # 81 of 404 = 20.0495: rounded up 20.1, to the nearest 20.0, as printed 20.05
        self.base({"internal/foo/a.go": shared(81, 404)}, {"internal/foo/a.go": "40.0"})
        code, out = self.r.run("--write-ledger")
        self.assertEqual(code, 0, out)
        led = self.r.read("tools/comment-ceilings.txt")
        m = re.search(r"^internal/foo/a\.go\s+(\S+)$", led, re.M)
        self.assertIsNotNone(m, led)
        self.assertEqual(m.group(1), "20.1", led)
        self.assertClean(self.gate())


if __name__ == "__main__":
    unittest.main()
