"""Independent QA for tools/check-comments.py against SPEC-010, VTT-050..058.

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

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = "tools/check-comments.py"
BOUND = 6
DEFAULT = 25.0
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

    def base(self, files, rows=None):
        for p, t in files.items():
            self.r.write(p, t)
        self.r.ledger(rows or {})
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


if __name__ == "__main__":
    unittest.main()
