"""Independent QA of tools/check-requirements-chain.py, derived from the promise
and not from the implementation.

Sources, the only statements of intent these tests treat as authoritative:
  SPEC    docs/specifications/008-requirement-ids-come-from-the-dispenser.md
  TICKET  docs/superpowers/specs/2026-09-23-joining-rules-and-the-chain-design.md
  REQ     the requirement handed to QA: TICKET's "Done looks like" item 3, its
          "Rules this puts on the system" (the chain), and the sentence "exit 0
          clean, 1 findings on stderr, 2 nothing scanned".

Each test names the sentence it pins in a comment. Every fixture register here
carries the tag QAX, never the project's own: SPEC, "How a test cites its
requirement": "Only the tag <the project's> reads as a claim about this
register; a fixture that builds registers as test data carries another tag."
The real tag is read at run time from the copied register and never written
into this file, because this file is itself one of the scanned test files.

Run: python3 tools/check_requirements_chain_qa_test.py [-q]
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CHECKER = os.path.join("tools", "check-requirements-chain.py")
DISPENSER = shutil.which("requirement-id") or os.path.expanduser(
    "~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id")

TAG = "QAX"
UNDEFINED = TAG + "-999"
DEFINED = TAG + "-001"
OPEN = "**OPEN — no test yet**"
READING_NAMED = "**READING — the 2026-09-23 reading review**"
READING_BARE = "**READING —**"

GO_TEST = """package fixture

import "testing"

{comment}func {name}(t *testing.T) {{}}
"""

PY_TEST = """import unittest


class Fixture(unittest.TestCase):
    {comment}def {name}(self):
        pass
"""

TS_TEST = """import {{ test }} from "bun:test";

{comment}test({quote}{name}{quote}, () => {{}});
"""


def run_checker(root):
    """Drive the checker as the promise states it: python3 <checker> [root]."""
    proc = subprocess.run(
        ["python3", CHECKER, root],
        cwd=REPO,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        timeout=120,
    )
    return proc.returncode, proc.stdout, proc.stderr


def register_text(tag=TAG, rows=(), project_line=True, header=True):
    lines = ["# Requirements", ""]
    if project_line:
        lines += ["project: %s" % tag, ""]
    lines += ["A fixture register built by QA.", ""]
    if header:
        lines += ["| Id | Requirement | Verified by |", "|---|---|---|"]
    lines += list(rows)
    return "\n".join(lines) + "\n"


def row(id_, evidence, text="A fixture rule"):
    return "| %s | %s | %s |" % (id_, text, evidence)


class Tree(object):
    """A synthetic repository laid out like the real one."""

    def __init__(self):
        self.root = tempfile.mkdtemp(prefix="qa-chain-")
        os.makedirs(os.path.join(self.root, "docs", "specifications"))

    def write(self, rel, content):
        path = os.path.join(self.root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        return rel

    def remove(self, rel):
        os.remove(os.path.join(self.root, rel))

    def register(self, **kw):
        return self.write("docs/requirements.md", register_text(**kw))

    def go_test(self, rel, cites=None, name="TestHeld"):
        comment = "// %s\n" % cites if cites else ""
        return self.write(rel, GO_TEST.format(comment=comment, name=name))

    def py_test(self, rel, cites=None, name="test_held"):
        comment = "# %s\n    " % cites if cites else ""
        return self.write(rel, PY_TEST.format(comment=comment, name=name))

    def ts_test(self, rel, cites=None, name="the door stays shut", quote='"'):
        comment = "// %s\n" % cites if cites else ""
        return self.write(
            rel, TS_TEST.format(comment=comment, name=name, quote=quote)
        )

    def spec(self, rel, names):
        return self.write(
            "docs/specifications/" + rel,
            "# SPEC-QA: a fixture record\n\n## Requirements\n\n%s\n" % names,
        )

    def run(self):
        return run_checker(self.root)


def baseline():
    """An empty register and one uncited test file: the smallest tree that
    scans something."""
    t = Tree()
    t.register()
    t.go_test("internal/fixture/held_test.go")
    return t


class Citations(unittest.TestCase):
    # REQ / TICKET "Done looks like" item 3: exits non-zero, naming the
    # offender, when "a test file under internal/, cmd/, client/, tools/ or
    # contract/ cites <TAG>-999 and no row defines it".
    # REQ, the sentence "exit 0 clean, 1 findings on stderr, 2 nothing scanned".
    def test_a_test_file_citing_an_undefined_id_is_refused_under_each_root(self):
        for root in ("internal", "cmd", "client", "tools", "contract"):
            with self.subTest(root=root):
                t = baseline()
                rel = t.go_test(root + "/fixture/cited_test.go", cites=UNDEFINED)
                code, out, err = t.run()
                self.assertEqual(code, 1, err)
                self.assertIn(UNDEFINED, err)
                self.assertIn(rel, err)

    # SPEC, "How the chain is checked": reads "every test file (`*_test.go`,
    # `*.test.ts`, `*_test.py`)" and "refuses a test or a specification that
    # cites an id no row defines".
    def test_each_test_file_kind_is_read_for_citations(self):
        cases = (
            ("go", lambda t: t.go_test("internal/fixture/cited_test.go", cites=UNDEFINED)),
            ("py", lambda t: t.py_test("tools/cited_test.py", cites=UNDEFINED)),
            ("ts", lambda t: t.ts_test("client/test/cited.test.ts", cites=UNDEFINED)),
        )
        for kind, place in cases:
            with self.subTest(kind=kind):
                t = baseline()
                rel = place(t)
                code, out, err = t.run()
                self.assertEqual(code, 1, err)
                self.assertIn(UNDEFINED, err)
                self.assertIn(rel, err)

    # REQ / TICKET item 3: "a file under docs/specifications/ names <TAG>-999
    # and no row defines it".
    def test_a_specification_naming_an_undefined_id_is_refused(self):
        t = baseline()
        rel = t.spec("010-fixture.md", UNDEFINED)
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(UNDEFINED, err)
        self.assertIn(rel, err)

    # TICKET, "Rules this puts on the system": "An id cited by a test or named
    # by a specification resolves to a row in the register." A row with the
    # dispenser's OPEN cell is a row; SPEC, "How it works": the evidence cell
    # may hold "`**OPEN — no test yet**`".
    def test_a_citation_resolves_to_a_row_whatever_the_rows_evidence(self):
        t = baseline()
        t.register(rows=[row(DEFINED, OPEN)])
        t.go_test("internal/fixture/cited_test.go", cites=DEFINED)
        t.spec("010-fixture.md", DEFINED)
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # SPEC, "How the chain is checked": "Only the register's own tag reads as
    # a citation."
    def test_another_projects_tag_is_data_not_a_citation(self):
        t = baseline()
        t.go_test("internal/fixture/rival_test.go", cites="RIVAL-999")
        t.py_test("tools/rival_test.py", cites="RIVAL-999")
        t.spec("010-fixture.md", "RIVAL-999")
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # SPEC, "How the chain is checked": the checker reads "every test file
    # (`*_test.go`, `*.test.ts`, `*_test.py`) under internal/, cmd/, client/,
    # tools/ and contract/". An enumerated list: a file of another name, or
    # under another root, is not on it. Read as an inference from the list.
    def test_a_file_that_is_not_a_test_file_under_the_roots_is_not_read(self):
        cases = (
            ("not a test file", "internal/fixture/held.go"),
            ("outside the roots", "scenarios/fixture/held_test.go"),
            ("outside the roots, docs", "docs/held_test.py"),
        )
        for label, rel in cases:
            with self.subTest(case=label):
                t = baseline()
                if rel.endswith(".py"):
                    t.py_test(rel, cites=UNDEFINED)
                else:
                    t.go_test(rel, cites=UNDEFINED)
                code, out, err = t.run()
                self.assertEqual(code, 0, err)


class Rows(unittest.TestCase):
    # REQ / TICKET item 3: "a row's evidence names a file that does not
    # exist". SPEC: an evidence entry is "`<repo-relative path>#<check name>`".
    def test_an_evidence_file_that_does_not_exist_is_refused(self):
        t = baseline()
        missing = "internal/fixture/missing_test.go"
        t.register(rows=[row(DEFINED, missing + "#TestHeld")])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(missing, err)

    # REQ / TICKET item 3: "one that does not carry the row's id".
    def test_an_evidence_file_that_does_not_carry_the_id_is_refused(self):
        t = baseline()
        rel = "internal/fixture/held_test.go"  # exists, declares TestHeld, cites nothing
        t.register(rows=[row(DEFINED, rel + "#TestHeld")])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(rel, err)

    # SPEC, "How the chain is checked": refuses an evidence entry that names
    # "a check the file does not declare".
    def test_an_evidence_file_that_does_not_declare_the_check_is_refused(self):
        t = baseline()
        rel = t.go_test("internal/fixture/cited_test.go", cites=DEFINED, name="TestHeld")
        t.register(rows=[row(DEFINED, rel + "#TestNope")])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(rel, err)

    # SPEC, "How the chain is checked": refuses "an evidence entry that names
    # no check"; the entry shape is "`<repo-relative path>#<check name>`".
    def test_an_evidence_entry_that_names_no_check_is_refused(self):
        t = baseline()
        rel = t.go_test("internal/fixture/cited_test.go", cites=DEFINED)
        t.register(rows=[row(DEFINED, rel)])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(rel, err)

    # SPEC, "How the chain is checked": "the check is a `func` in a Go file,
    # a `def` in a Python file, or the quoted test name in a TypeScript file";
    # "entries comma-separated". All three kinds, in one cell, hold one row.
    def test_a_check_is_a_go_func_a_python_def_or_a_quoted_ts_name(self):
        t = baseline()
        go = t.go_test("internal/fixture/cited_test.go", cites=DEFINED, name="TestHeld")
        py = t.py_test("tools/held_test.py", cites=DEFINED, name="test_held")
        ts = t.ts_test("client/test/held.test.ts", cites=DEFINED, name="the door stays shut")
        evidence = "%s#TestHeld, %s#test_held, %s#the door stays shut" % (go, py, ts)
        t.register(rows=[row(DEFINED, evidence)])
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # SPEC: "the quoted test name in a TypeScript file". A name the file does
    # not quote is "a check the file does not declare".
    def test_a_ts_name_the_file_does_not_quote_is_refused(self):
        t = baseline()
        ts = t.ts_test("client/test/held.test.ts", cites=DEFINED, name="the door stays shut")
        t.register(rows=[row(DEFINED, ts + "#the door swings open")])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(ts, err)

    # SPEC: "entries comma-separated". Each entry is checked: one bad entry
    # among good ones is still refused, and the bad one is the one named.
    def test_each_comma_separated_entry_is_checked(self):
        t = baseline()
        good = t.go_test("internal/fixture/good_test.go", cites=DEFINED, name="TestGood")
        bad = t.go_test("internal/fixture/bad_test.go", cites=None, name="TestBad")
        t.register(rows=[row(DEFINED, "%s#TestGood, %s#TestBad" % (good, bad))])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(bad, err)

    # REQ / TICKET item 3: "a row's id is not of the shape <TAG>-NNN".
    # TICKET, rules: "A row's id is the project tag and a number".
    def test_a_row_whose_id_is_not_the_tag_and_a_number_is_refused(self):
        cases = (
            ("per-subject segment", TAG + "-AUTH-001"),
            ("another tag", "RIVAL-001"),
            ("not three digits", TAG + "-1"),
        )
        for label, bad_id in cases:
            with self.subTest(case=label):
                t = baseline()
                t.register(rows=[row(bad_id, OPEN)])
                code, out, err = t.run()
                self.assertEqual(code, 1, err)
                self.assertIn(bad_id, err)

    # SPEC, "How the chain is checked": "a row whose id is not <TAG>-NNN once
    # markup is stripped" -- so an id wrapped in markup is read through, both
    # as a definition and as the id the evidence file must carry.
    def test_an_id_wrapped_in_markup_is_read_through(self):
        for label, wrapped in (("backticks", "`%s`"), ("bold", "**%s**")):
            with self.subTest(case=label):
                t = baseline()
                rel = t.go_test("internal/fixture/cited_test.go", cites=DEFINED)
                t.register(rows=[row(wrapped % DEFINED, rel + "#TestHeld")])
                code, out, err = t.run()
                self.assertEqual(code, 0, err)
                self.assertEqual(err, "")

    # REQ / TICKET item 3: "two rows share an id". TICKET, rules: "no two rows
    # share one".
    def test_two_rows_sharing_an_id_are_refused(self):
        t = baseline()
        t.register(rows=[row(DEFINED, OPEN, "First"), row(DEFINED, OPEN, "Second")])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(DEFINED, err)

    # REQ / TICKET item 3: "an evidence cell is blank".
    def test_a_blank_evidence_cell_is_refused(self):
        for label, cell in (("empty", ""), ("spaces", "   ")):
            with self.subTest(case=label):
                t = baseline()
                t.register(rows=[row(DEFINED, cell)])
                code, out, err = t.run()
                self.assertEqual(code, 1, err)
                self.assertIn(DEFINED, err)

    # SPEC, "How the chain is checked": refuses "a `READING` that names no
    # review".
    def test_a_reading_that_names_no_review_is_refused(self):
        t = baseline()
        t.register(rows=[row(DEFINED, READING_BARE)])
        code, out, err = t.run()
        self.assertEqual(code, 1, err)
        self.assertIn(DEFINED, err)

    # SPEC, "How it works": the evidence cell may hold "`**READING — <the
    # review>**`".
    def test_a_reading_that_names_its_review_passes(self):
        t = baseline()
        t.register(rows=[row(DEFINED, READING_NAMED)])
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # SPEC, "How it works": "Only the dispenser writes an id." A register the
    # dispenser wrote, with the dispenser's own OPEN cells, passes.
    @unittest.skipUnless(os.access(DISPENSER, os.X_OK), "dispenser not installed")
    def test_a_register_the_dispenser_wrote_passes(self):
        t = baseline()
        reg = os.path.join(t.root, "docs", "requirements.md")
        for sentence in ("A first fixture rule", "A second fixture rule"):
            proc = subprocess.run(
                [DISPENSER, "--register", reg, sentence],
                cwd=t.root,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                universal_newlines=True,
            )
            self.assertEqual(proc.returncode, 0, proc.stderr)
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")


class NothingScanned(unittest.TestCase):
    # REQ: "exit 0 clean, 1 findings on stderr, 2 nothing scanned". SPEC: "a
    # run that scans nothing fails". No register: nothing to scan against.
    def test_no_register_exits_2(self):
        t = Tree()
        t.go_test("internal/fixture/held_test.go")
        code, out, err = t.run()
        self.assertEqual(code, 2, err)

    # REQ: "2 nothing scanned". A register with nothing to scan around it: no
    # test file under any root and no specification.
    def test_no_test_files_and_no_specifications_exits_2(self):
        t = Tree()
        t.register()
        code, out, err = t.run()
        self.assertEqual(code, 2, err)

    # SPEC, "How it works": "The tag is declared once, on the register's
    # `project:` line." Without it there is no tag to read citations by, so
    # the run cannot be clean. Which non-zero code is not determined by the
    # spec; the assertion is only that it is not 0.
    def test_a_register_without_a_project_line_is_not_clean(self):
        t = baseline()
        t.register(project_line=False)
        code, out, err = t.run()
        self.assertNotEqual(code, 0, out)

    # SPEC, "How the chain is checked": "An empty register passes".
    def test_an_empty_register_passes(self):
        t = baseline()
        code, out, err = t.run()
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")


class RealTree(unittest.TestCase):
    # REQ / TICKET item 3: "and exits zero on the committed tree". REQ: "exit
    # 0 clean, 1 findings on stderr".
    def test_the_committed_tree_passes_with_nothing_on_stderr(self):
        code, out, err = run_checker(REPO)
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # REQ / TICKET item 3: "`task check` has a step that ...". A dry run of
    # `task check` lists the checker among what it would run.
    @unittest.skipUnless(shutil.which("task"), "task not installed")
    def test_the_step_is_among_task_checks_steps(self):
        proc = subprocess.run(
            ["task", "check", "--dry"],
            cwd=REPO,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
            timeout=120,
        )
        listing = proc.stdout + proc.stderr
        self.assertIn("check:requirements-chain", listing)
        self.assertIn("check-requirements-chain.py", listing)


class RealTreeInjected(unittest.TestCase):
    """TICKET item 3: "Each refusal is observed by injecting its case on the
    real tree and reverting it." Done on a copy of the real tree's scanned
    roots, so the repository is never edited; the copy carries the real
    register, its real tag and the real test files."""

    ROOTS = ("docs", "internal", "cmd", "client", "tools", "contract")
    IGNORE = shutil.ignore_patterns(
        "node_modules", ".git", ".stryker-tmp", ".tscov", ".superpowers"
    )

    @classmethod
    def setUpClass(cls):
        cls.root = tempfile.mkdtemp(prefix="qa-chain-real-")
        for d in cls.ROOTS:
            src = os.path.join(REPO, d)
            if os.path.isdir(src):
                shutil.copytree(src, os.path.join(cls.root, d), ignore=cls.IGNORE)
        cls.register = os.path.join(cls.root, "docs", "requirements.md")
        with open(cls.register, encoding="utf-8") as f:
            cls.pristine = f.read()
        m = re.search(r"^project:\s*(\S+)\s*$", cls.pristine, re.M)
        assert m, "the copied register declares no tag"
        cls.tag = m.group(1)

    def tearDown(self):
        with open(self.register, "w", encoding="utf-8") as f:
            f.write(self.pristine)
        for rel in getattr(self, "added", ()):
            path = os.path.join(self.root, rel)
            if os.path.exists(path):
                os.remove(path)

    def add(self, rel, content):
        self.added = getattr(self, "added", []) + [rel]
        path = os.path.join(self.root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        return rel

    def rows(self, *rows):
        with open(self.register, "a", encoding="utf-8") as f:
            f.write("\n".join(rows) + "\n")

    def id_(self, n):
        return "%s-%03d" % (self.tag, n)

    # TICKET item 3: "and exits zero on the committed tree", on the copy.
    def test_the_untouched_copy_passes(self):
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 0, err)
        self.assertEqual(err, "")

    # TICKET item 3: "a test file under internal/ ... cites <TAG>-999 and no
    # row defines it", with the project's own tag, on the real test tree.
    def test_a_test_file_citing_the_projects_undefined_id_is_refused(self):
        cited = self.id_(999)
        rel = self.add(
            "internal/gateway/qa_injected_test.go",
            GO_TEST.format(comment="// %s\n" % cited, name="TestInjected"),
        )
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(cited, err)
        self.assertIn(rel, err)

    # TICKET item 3: "a file under docs/specifications/ names <TAG>-999 and no
    # row defines it".
    def test_a_specification_naming_the_projects_undefined_id_is_refused(self):
        cited = self.id_(999)
        rel = self.add(
            "docs/specifications/999-qa-injected.md",
            "# SPEC-999: injected\n\n## Requirements\n\n%s\n" % cited,
        )
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(cited, err)
        self.assertIn(rel, err)

    # TICKET item 3: "a row's evidence names a file that does not exist".
    def test_a_row_naming_a_missing_file_is_refused(self):
        missing = "internal/gateway/nowhere_test.go"
        self.rows(row(self.id_(1), missing + "#TestNowhere"))
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(missing, err)

    # TICKET item 3: "or one that does not carry the row's id". The file is a
    # real one and the check a real one, named by TICKET item 6:
    # TestARefusedJoinWritesNothingAtAll.
    def test_a_row_naming_a_real_file_that_does_not_carry_the_id_is_refused(self):
        check = "TestARefusedJoinWritesNothingAtAll"
        rel = "internal/gateway/join_test.go"
        with open(os.path.join(self.root, rel), encoding="utf-8") as f:
            body = f.read()
        self.assertIn("func %s(" % check, body)
        self.assertNotIn(self.id_(1), body)
        self.rows(row(self.id_(1), rel + "#" + check))
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(rel, err)

    # TICKET item 3: "a row's id is not of the shape <TAG>-NNN".
    def test_a_row_of_the_old_per_subject_shape_is_refused(self):
        bad = self.tag + "-AUTH-001"
        self.rows(row(bad, OPEN))
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(bad, err)

    # TICKET item 3: "two rows share an id".
    def test_two_rows_sharing_the_projects_id_are_refused(self):
        self.rows(row(self.id_(1), OPEN, "First"), row(self.id_(1), OPEN, "Second"))
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(self.id_(1), err)

    # TICKET item 3: "an evidence cell is blank".
    def test_a_blank_evidence_cell_on_the_projects_register_is_refused(self):
        self.rows(row(self.id_(1), ""))
        code, out, err = run_checker(self.root)
        self.assertEqual(code, 1, err)
        self.assertIn(self.id_(1), err)


if __name__ == "__main__":
    unittest.main()
