#!/usr/bin/env python3
"""Tests for check-new-prose.py, the scoping wrapper.

WHAT IS WORTH TESTING HERE is the SCOPING, not the two checkers — each of those
has its own test. The wrapper's whole reason to exist is that it holds a change
to its own added lines and to nobody else's, so that is what these pin.

BOTH HALVES GET THE SAME PAIR: one test that an added defect fails, one that the
same defect left alone in a touched file does not. An earlier version of this
file tested only the wrap half, and every assertion in it stayed green while the
citations half was scanning one directory and silently skipping the rest.
"""

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parent / "check-new-prose.py"


def git(repo, *args):
    return subprocess.run(["git", "-C", str(repo), *args],
                          capture_output=True, text=True, check=True).stdout


class NewProseTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Path(self.tmp.name)
        git(self.repo, "init", "-q")
        git(self.repo, "config", "user.email", "t@example.com")
        git(self.repo, "config", "user.name", "t")
        # The tool shells out to the checkers by relative path, so it has to run
        # from a tree that has them.
        (self.repo / "tools").mkdir()
        for name in ("check-citations.py", "check-comment-wrap.py"):
            (self.repo / "tools" / name).write_bytes(
                (TOOL.parent / name).read_bytes())
        (self.repo / "tools" / "check-new-prose.py").write_bytes(TOOL.read_bytes())

    def tearDown(self):
        self.tmp.cleanup()

    def run_tool(self, base="HEAD"):
        return subprocess.run([sys.executable, "tools/check-new-prose.py", base],
                              cwd=str(self.repo), capture_output=True, text=True)

    def write(self, name, text):
        (self.repo / name).write_text(text, encoding="utf-8")

    def commit(self, msg="c"):
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", msg)

    def test_a_clean_change_passes(self):
        self.write("a.go", "package a\n\n// A is fine.\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\n// A is fine.\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)

    def test_an_over_long_added_comment_fails(self):
        self.write("a.go", "package a\n\n// A is fine.\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\n// A is fine.\nfunc A() {}\n\n// " +
                   "x" * 100 + " and more words to carry the line past the band.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1, got.stdout + got.stderr)

    def test_an_existing_finding_in_a_touched_file_does_not_fail_it(self):
        """THE POINT OF THE WHOLE WRAPPER.

        A file that already carries a finding must stay editable: scoping to
        FILES would make somebody else's older line this change's problem.
        """
        bad = "// " + "y" * 100 + " and more words to carry the line past the band.\n"
        self.write("a.go", "package a\n\n" + bad + "func A() {}\n")
        self.commit()
        # Touch the file WITHOUT touching the offending line.
        self.write("a.go", "package a\n\n" + bad + "func A() {}\n\n// B is fine.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "an older finding in a touched file must not fail the change that "
                         "left it alone:\n" + got.stdout + got.stderr)

    def test_a_new_untracked_file_is_wholly_in_scope(self):
        """git diff cannot see an untracked file, and a new file is all new."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("b.go", "package a\n\n// " + "z" * 100 +
                   " and more words to carry the line past the band.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a brand-new file's prose was not held:\n" + got.stdout + got.stderr)

    def test_no_change_is_not_a_failure(self):
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        got = self.run_tool()
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertIn("nothing added", got.stdout)

    # ---- the citations half ------------------------------------------------

    def test_a_fabricated_citation_on_an_added_line_fails(self):
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\nfunc A() {}\n\n"
                   "// B is pinned by TestNothingHasEverDeclaredThis.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a cited name this repo never declared was not held:\n"
                         + got.stdout + got.stderr)
        self.assertIn("TestNothingHasEverDeclaredThis", got.stderr)

    def test_an_existing_fabricated_citation_in_a_touched_file_does_not_fail_it(self):
        """The same scoping promise the wrap half makes, for citations.

        This is the assertion that a per-directory scan cannot keep and that no
        test here made until the scan became whole-tree-then-filter.
        """
        bad = "// A is pinned by TestNothingHasEverDeclaredThis.\n"
        self.write("a.go", "package a\n\n" + bad + "func A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\n" + bad + "func A() {}\n\n// B is fine.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "an older citation finding in a touched file must not fail the "
                         "change that left it alone:\n" + got.stdout + got.stderr)

    def test_a_name_this_change_declares_elsewhere_resolves(self):
        """A citation across directories, to a name added by this same change.

        Nothing in git history declares it yet, so the only way it resolves is a
        scan wide enough to see the file declaring it. A scan scoped to the
        citing file's own directory reports it as a fabrication.
        """
        (self.repo / "pkga").mkdir()
        (self.repo / "pkgb").mkdir()
        self.write("pkga/a.go", "package pkga\n\nfunc A() {}\n")
        self.commit()
        self.write("pkgb/b.go", "package pkgb\n\nfunc TestFreshlyDeclaredHere() {}\n")
        self.write("pkga/a.go", "package pkga\n\nfunc A() {}\n\n"
                   "// B is pinned by TestFreshlyDeclaredHere.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "a name declared by this very change was called a fabrication:\n"
                         + got.stdout + got.stderr)

    def test_a_change_with_no_go_file_does_not_pay_for_the_citation_scan(self):
        """Citations are read from Go comments only, so a TypeScript-only change
        has no path the scan could report a finding at. Proven by breaking the
        checker: if it ran at all, the crash guard would fail this. The file is
        .ts rather than .md deliberately — a .md file is out of scope for BOTH
        halves, so it would pass without proving anything about the skip."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        (self.repo / "client" / "src").mkdir(parents=True)
        self.write("client/src/b.ts", "// A fine comment of an entirely ordinary length here.\n"
                                      "export const b = 1\n")
        self.break_citations("raise RuntimeError('this must never run')\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "a change with no Go file in it ran the citation scan:\n"
                         + got.stdout + got.stderr)

    def test_a_change_that_does_touch_go_still_pays(self):
        """The control for the skip above: without it the skip could be
        unconditional and both tests would still pass."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        self.break_citations("raise RuntimeError('this must run')\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a Go change skipped the citation scan:\n" + got.stdout + got.stderr)

    # ---- a checker that did not run is not a clean tree --------------------

    def break_citations(self, body):
        (self.repo / "tools" / "check-citations.py").write_text(body, encoding="utf-8")

    def test_a_crashing_checker_is_not_read_as_a_clean_tree(self):
        """A traceback exits 1, and 1 is also 'I found findings'.

        So the exit code alone cannot tell the two apart, and a wrapper that
        trusts it reports a clean tree for a checker that never ran.
        """
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        self.break_citations("raise RuntimeError('boom')\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a checker that crashed was reported as clean:\n"
                         + got.stdout + got.stderr)
        self.assertIn("did not run", got.stderr)

    def test_a_checker_that_refuses_to_scan_is_not_read_as_a_clean_tree(self):
        """check-citations.py exits 2 to say 'nothing was scanned, so nothing is
        proven'. That is the one sentence it exists to be able to say."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        self.break_citations("import sys\nsys.exit(2)\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a refusal to scan was reported as clean:\n"
                         + got.stdout + got.stderr)

    def test_an_exit_code_that_is_not_a_verdict_fails_even_when_it_finished(self):
        """The completion line and the exit code are separate contracts. Without
        this, a checker could print a clean summary and exit on an error path."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        self.break_citations("import sys\n"
                             "print('check:citations: 1 files, every cited name has existed.')\n"
                             "sys.exit(3)\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a checker that finished and then errored was read as clean:\n"
                         + got.stdout + got.stderr)

    # ---- what counts as a file header --------------------------------------

    FILLER = "// a filler comment line of a perfectly ordinary and unremarkable width."

    def test_an_added_line_that_looks_like_a_diff_header_does_not_move_the_scope(self):
        """Under --unified=0 an added line beginning "++ " renders as "+++ ...".
        Read as a header it reassigns every LATER hunk to a path that does not
        exist, and the gate reports a clean tree for prose it never looked at.

        The fixture is built so the only finding sits in the SECOND hunk — a
        finding in the first would be attributed correctly even by the broken
        parser, and the test would pass while proving nothing. The quoted line
        sits in a raw string rather than a comment so it contributes no finding
        of its own; this repository's tests and reports quote diffs.
        """
        body = ["package a", ""] + [self.FILLER] * 40
        self.write("a.go", "\n".join(body) + "\nfunc A() {}\n")
        self.commit()
        spliced = (body[:2] + ["var diffQuote = `", "++ b/phantom.go", "`"] + body[2:]
                   + [self.FILLER,
                      "// " + "x" * 100 + " and more words to carry this one past the band."])
        self.write("a.go", "\n".join(spliced) + "\nfunc A() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a line that merely LOOKS like a diff header hid a real finding:\n"
                         + got.stdout + got.stderr)
        self.assertNotIn("phantom.go", got.stdout + got.stderr)

    # ---- what is in scope at all -------------------------------------------

    GEN_HEAD = ("package gen\n\n"
                "// Types that are valid to be assigned to Payload are enumerated just below,\n")
    GEN_TAIL = "// and the enumeration keeps going afterwards with further entries after it.\n"

    def test_a_generated_tree_is_not_anybody_s_prose(self):
        """protoc-gen-go writes short lines into its own doc blocks. Adding one
        event — the only contract change CLAUDE.md rule 3 permits — adds one such
        line, and there is no legal remedy: a hand-edit under gen/ is overwritten
        by regeneration and then fails check:drift. Two generators do it, not
        one — protoc-gen-go in the .pb.go and protoc-gen-es in the .ts. TRACKED and modified, so it
        travels the diff path rather than the untracked one."""
        (self.repo / "contract" / "gen").mkdir(parents=True)
        self.write("contract/gen/events.pb.go", self.GEN_HEAD + self.GEN_TAIL)
        self.commit()
        self.write("contract/gen/events.pb.go",
                   self.GEN_HEAD + "//\t*Envelope_WidgetAdded\n" + self.GEN_TAIL)
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "generated code was held to a prose band:\n" + got.stdout + got.stderr)

    def test_the_generated_shape_is_flagged_anywhere_else(self):
        """The control for the exemption above. Without it, that test would pass
        for a fixture the checker never flags at all."""
        self.write("live.go", self.GEN_HEAD.replace("package gen", "package a") + self.GEN_TAIL)
        self.commit()
        self.write("live.go", self.GEN_HEAD.replace("package gen", "package a")
                   + "//\t*Envelope_WidgetAdded\n" + self.GEN_TAIL)
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "the fixture is not flagged outside gen/, so the exemption test "
                         "proves nothing:\n" + got.stdout + got.stderr)

    LONG_HEADING = ("## A heading long enough to exceed the upper edge of the band all by "
                    "itself, without any help at all\n")

    def test_markdown_is_held_by_neither_half(self):
        """`^\\s*(//+|\\*|#)` is a comment marker in Go and TypeScript and something
        else in Markdown: a heading, or the first star of **bold**. Both the
        tracked and the untracked path are exercised, because they scope
        separately and an earlier version filtered only one of them."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.write("tracked.md", "# A spec\n")
        self.commit()
        self.write("tracked.md", "# A spec\n\n" + self.LONG_HEADING)
        self.write("untracked.md", "# Another\n\n" + self.LONG_HEADING)
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "a Markdown heading was held to a Go comment band:\n"
                         + got.stdout + got.stderr)
        self.assertIn("nothing added", got.stdout)

    def test_the_hatch_must_open_the_comment_not_merely_appear_in_it(self):
        """An exemption is a clean verdict, so it takes the narrow test. A line
        that MENTIONS a hatch mid-sentence — as check-citations.py's own remedy
        text does — must still be held. The two lines around it are inside the
        band, so the mentioning line is the only finding and the assertion cannot
        be satisfied by a neighbour."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go",
                   "package a\n\n"
                   "// A is described here by a first line of a perfectly ordinary width,\n"
                   "// adjudicate it with a citations:ok line\n"
                   "// and the block then continues afterwards with another line of width.\n"
                   "func A() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a line that merely mentions a hatch was exempted:\n"
                         + got.stdout + got.stderr)

    # ---- scope ------------------------------------------------------------

    def test_a_missing_base_ref_fails_and_says_what_to_do(self):
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        got = self.run_tool(base="origin/nowhere")
        self.assertEqual(got.returncode, 1, got.stdout + got.stderr)
        self.assertIn("git fetch", got.stderr)

    def test_the_base_ref_falls_back_to_the_origin_spelling(self):
        """check:breaking carries the scar this copies: actions/checkout creates a
        local branch only for the ref it checks out, so on a pull request `main`
        exists solely as refs/remotes/origin/main. That gate hard-failed on this
        repository's FIRST pull request. This one has never run in CI at all."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        head = git(self.repo, "rev-parse", "HEAD").strip()
        local = git(self.repo, "rev-parse", "--abbrev-ref", "HEAD").strip()
        git(self.repo, "update-ref", "refs/remotes/origin/main", head)
        git(self.repo, "checkout", "-q", "-b", "feature")
        git(self.repo, "branch", "-q", "-D", local)  # only the origin/ spelling survives
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        got = self.run_tool(base="main")
        self.assertEqual(got.returncode, 0,
                         "the gate took one spelling of the base ref on faith:\n"
                         + got.stdout + got.stderr)

    def test_lines_the_base_branch_removed_are_not_this_change_s_problem(self):
        """Two-dot `git diff <base>` compares endpoints, so a file the BASE
        deleted reappears as this branch's addition. The branch never wrote it."""
        bad = "// " + "q" * 100 + " and more words to carry the line past the band.\n"
        self.write("old.go", "package a\n\n" + bad + "func Old() {}\n")
        self.commit()
        base = git(self.repo, "rev-parse", "--abbrev-ref", "HEAD").strip()
        git(self.repo, "branch", "feature")
        (self.repo / "old.go").unlink()
        self.commit("base deletes it")
        git(self.repo, "checkout", "-q", "feature")
        self.write("a.go", "package a\n\n// A is fine.\nfunc A() {}\n")
        got = self.run_tool(base=base)
        self.assertEqual(got.returncode, 0,
                         "the branch was held answerable for a line the base removed:\n"
                         + got.stdout + got.stderr)

    # ---- git config is not this gate's to trust ----------------------------

    LONG_LINE = "// " + "x" * 100 + " and more words to carry this one past the band.\n"

    def test_an_external_differ_does_not_empty_the_scope(self):
        """GIT_EXTERNAL_DIFF / diff.external makes `git diff` emit no
        `diff --git` headers at all, so every line of this change becomes
        invisible and the gate reports a clean tree. difftastic and delta setups
        are ordinary; the gate must not depend on the developer's git config."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        git(self.repo, "config", "diff.external", "/usr/bin/true")
        self.write("a.go", "package a\n\nfunc A() {}\n\n" + self.LONG_LINE + "func B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "an external differ emptied the scope and the gate went green:\n"
                         + got.stdout + got.stderr)

    def test_a_prefixless_diff_config_does_not_break_the_gate(self):
        """diff.noprefix drops the `b/`, and mnemonicPrefix replaces it. Without
        normalising, every path fails the `b/` test and lands in the quoted-path
        refusal — whose remedy is "rename it", for a file with an ordinary name.
        Unpassable for that developer, with no hatch that reaches it."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        git(self.repo, "config", "diff.noprefix", "true")
        self.write("a.go", "package a\n\nfunc A() {}\n\n// B is fine.\nfunc B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "an ordinary git config made the gate unpassable:\n"
                         + got.stdout + got.stderr)

    def test_a_tracked_path_containing_a_space_is_still_held(self):
        """Git appends a TAB to the `+++` header to disambiguate such a path, so
        the name no longer ends in a checked extension: the file is dropped AND
        the refusal written for hostile paths never fires."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        (self.repo / "my dir").mkdir()
        self.write("my dir/b.go", "package a\n\n" + self.LONG_LINE + "func B() {}\n")
        git(self.repo, "add", "-A")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a tracked path with a space was passed over in silence:\n"
                         + got.stdout + got.stderr)

    def test_an_untracked_path_containing_a_space_is_still_held(self):
        """Scope is right here (ls-files -z), but the finding the checker PRINTED
        was discarded, because the wrap pattern stopped at the space."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        (self.repo / "my dir").mkdir()
        self.write("my dir/b.go", "package a\n\n" + self.LONG_LINE + "func B() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a finding on an untracked path with a space was discarded:\n"
                         + got.stdout + got.stderr)

    # ---- the hatch --------------------------------------------------------

    HATCH_BLOCK = ("package a\n\n"
                   "// A resolves a square the way MapTool's Zone does, one grid size per\n"
                   "// map, which is where the number actually varies between two scenes.\n"
                   "// %s\n"
                   "// The rest of the block keeps going after that line, which is the shape\n"
                   "// that makes a short line read as a bad splice rather than a paragraph.\n"
                   "func A() {}\n")

    def test_a_hatched_line_is_not_held_to_the_prose_band(self):
        """CLAUDE.md rule 9 mandates citing MapTool's names, which this repository
        will never declare, and check-citations.py's remedy is to append its hatch
        — a fixed-format annotation the wrap half then reads as a bad splice. The
        only escape without this exemption is ending the hatch with a period,
        which nobody would guess."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", self.HATCH_BLOCK % "citations:ok MapTool name, not ours")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0,
                         "the documented remedy for rule 9 failed the gate:\n"
                         + got.stdout + got.stderr)

    def test_the_same_line_without_a_hatch_is_held(self):
        """The control. Without it the test above passes for any reason at all —
        including a wrap half that stopped flagging this shape."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write("a.go", self.HATCH_BLOCK % "a plain short line, no hatch on it")
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "the exemption is vacuous: this shape is not flagged at all:\n"
                         + got.stdout + got.stderr)

    def test_a_file_that_only_lost_lines_is_not_in_scope(self):
        """A pure-deletion hunk reads as `+9,0` — an EMPTY range that still names
        a file. Keeping that key makes the gate claim a scope it does not have,
        and hands the wrap half a path with nothing added in it."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.write("b.go", "package a\n\n// B is fine.\n// B is still fine.\nfunc B() {}\n")
        self.commit()
        self.write("b.go", "package a\n\n// B is fine.\nfunc B() {}\n")
        self.write("a.go", "package a\n\nfunc A() {}\n\n// C is fine.\nfunc C() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertIn("across 1 file(s)", got.stdout)

    def test_a_path_git_has_to_quote_fails_rather_than_being_skipped(self):
        """core.quotePath=false unquotes non-ASCII names, not every name. A path
        holding a quote still arrives escaped, and the `+++` line no longer parses
        — so the file's added lines would be invisible, and invisible reads as
        clean. The refusal is the point; the exotic filename is just how to reach
        it."""
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.commit()
        self.write('we"ird.go', "package a\n\n// " + "w" * 100 +
                   " and more words to carry the line past the band.\nfunc W() {}\n")
        git(self.repo, "add", "-A")  # tracked, so it travels through the diff
        got = self.run_tool()
        self.assertEqual(got.returncode, 1,
                         "a file the gate could not parse was passed over in silence:\n"
                         + got.stdout + got.stderr)
        self.assertIn("had to quote", got.stderr)

    def test_a_deleted_file_is_not_in_scope(self):
        self.write("a.go", "package a\n\nfunc A() {}\n")
        self.write("b.go", "package a\n\nfunc B() {}\n")
        self.commit()
        (self.repo / "b.go").unlink()
        self.write("a.go", "package a\n\nfunc A() {}\n\n// C is fine.\nfunc C() {}\n")
        got = self.run_tool()
        self.assertEqual(got.returncode, 0, got.stdout + got.stderr)
        self.assertIn("across 1 file(s)", got.stdout)


if __name__ == "__main__":
    unittest.main(verbosity=2)
