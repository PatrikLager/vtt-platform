#!/usr/bin/env python3
"""Boundary tests for check-citations.py.

A gate ships with "passes when it should fail" holes unless its own boundary is
tested — the standing lesson from the coverage gate's five review rounds. These
drive the checker over synthetic trees and assert WHICH findings come back.

THE ORACLE IS INJECTED, never the real git. scan() takes it as a parameter for
exactly this reason: a test that ran `git log --all -S` against this repository
would assert about its history rather than about the checker, would be slow,
and would change its answer every time somebody committed. The tests below hand
it a set of names "history knows" and drive both verdicts deliberately.

Run: python3 tools/check_citations_test.py
"""

import importlib.util
import io
import os
import pathlib
import shutil
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

_spec = importlib.util.spec_from_file_location(
    "check_citations",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-citations.py"))
cc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cc)


def tree(**files):
    """A temp dir of files; returns its path. Keys use __ for a path separator."""
    d = tempfile.mkdtemp()
    for name, body in files.items():
        p = pathlib.Path(d) / name.replace("__", "/")
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")
    return d


def oracle_knowing(*names):
    """An oracle that says history contains exactly names."""
    known = set(names)
    return lambda name, root: name in known


ORACLE_BLIND = lambda name, root: False   # history knows nothing
ORACLE_OMNISCIENT = lambda name, root: True  # history knows everything


class FabricationTests(unittest.TestCase):
    def scan(self, root, oracle=ORACLE_BLIND):
        return cc.scan(root, oracle=oracle)

    def test_a_cited_test_that_never_existed_is_a_finding(self):
        d = tree(**{"pkg__a_test.go": (
            "package pkg\n"
            "// See TestSecretsAreNeverLogged for the precedent.\n"
            "func TestSomethingElse(t *testing.T) {}\n")})
        try:
            _, fab, hist = self.scan(d)
            self.assertEqual([n for _, _, n in fab], ["TestSecretsAreNeverLogged"])
            self.assertEqual(hist, [])
        finally:
            shutil.rmtree(d)

    def test_a_cited_test_that_exists_is_not_a_finding(self):
        """The whole gate is worthless if it cannot see a declaration."""
        d = tree(**{"pkg__a_test.go": (
            "package pkg\n"
            "// See TestTheRealOne for the precedent.\n"
            "func TestTheRealOne(t *testing.T) {}\n")})
        try:
            _, fab, hist = self.scan(d)
            self.assertEqual(fab, [])
            self.assertEqual(hist, [])
        finally:
            shutil.rmtree(d)

    def test_a_deleted_name_history_remembers_is_historical_not_fatal(self):
        """An obituary is CORRECT prose. This is the distinction the oracle buys,
        and without it the gate would flag every 'X is GONE' block in the tree."""
        d = tree(**{"pkg__a.go": (
            "package pkg\n"
            "// ErrPackNotLoaded is gone, and its absence is the point.\n"
            "func Live() {}\n")})
        try:
            _, fab, hist = self.scan(d, oracle_knowing("ErrPackNotLoaded"))
            self.assertEqual(fab, [])
            self.assertEqual([n for _, _, n in hist], ["ErrPackNotLoaded"])
        finally:
            shutil.rmtree(d)

    def test_the_same_name_flips_verdict_with_the_oracle_alone(self):
        """Nothing but history separates a fabrication from an obituary — if this
        passes under both oracles, the oracle is not being consulted."""
        src = ("package pkg\n"
               "// handlePackFileThing used to live here.\n"
               "func Live() {}\n")
        d = tree(**{"pkg__a.go": src})
        try:
            _, fab, hist = self.scan(d, ORACLE_BLIND)
            self.assertEqual(len(fab), 1)
            self.assertEqual(hist, [])
            _, fab2, hist2 = self.scan(d, ORACLE_OMNISCIENT)
            self.assertEqual(fab2, [])
            self.assertEqual(len(hist2), 1)
        finally:
            shutil.rmtree(d)


class NeedleTests(unittest.TestCase):
    def cands(self, text, packages=frozenset({"mapdef"})):
        return cc.candidates(text, packages)

    def test_shape_two_needs_a_real_package_on_the_left(self):
        """mapdef.LoadPack is a citation; example.com/Whatever is not."""
        self.assertIn("LoadPack", self.cands("see mapdef.LoadPack for why"))
        self.assertNotIn("LoadPack", self.cands("see notapackage.LoadPack for why"))

    def test_a_two_segment_bare_word_is_not_a_needle(self):
        """The measured threshold: PackTile and ArtDir appear in ordinary English
        sentences about them, so two segments would make the gate unusable."""
        self.assertEqual(self.cands("the PackTile shape and its ArtDir"), set())

    def test_a_three_segment_bare_word_is_a_needle(self):
        """This is what catches handlePackFile and WithPackFiles, the names this
        branch left in comments after deleting them."""
        for name in ("handlePackFile", "WithPackFiles", "ResolveObjectArt"):
            self.assertIn(name, self.cands(f"see {name} above"))

    def test_a_go_stdlib_name_is_exempted_by_word(self):
        """Exempt by WORD, never by file — check-no-create-scene.py's discipline:
        exempting a file hides the next defect in that file."""
        self.assertEqual(self.cands("http.ServeFileFS does the rest"), set())

    def test_prose_alone_produces_no_needle(self):
        """A gate that fires on English gets switched off."""
        self.assertEqual(
            self.cands("This comment explains why the loader refuses a pack file."),
            set())


class WrapFragmentTests(unittest.TestCase):
    def test_a_mid_identifier_wrap_is_not_a_citation(self):
        """Go comments wrap at 80 columns mid-word with no marker. Both halves
        are undeclared, and without is_wrap_fragment a half with no history is
        reported as a fabrication — a false FAIL, the kind that kills a gate."""
        d = tree(**{"pkg__a_test.go": (
            "package pkg\n"
            "// internal/artlib's TestNoLookupErrorNamesThe\n"
            "// DirectoryItRead walks syscall phases.\n"
            "func TestNoLookupErrorNamesTheDirectoryItRead(t *testing.T) {}\n")})
        try:
            _, fab, hist = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [], "a wrapped declared name must not be a finding")
            self.assertEqual(hist, [])
        finally:
            shutil.rmtree(d)

    def test_a_fragment_of_nothing_is_still_a_finding(self):
        """The narrowness of the rule: only a prefix/suffix of a name the tree
        REALLY declares is forgiven. Otherwise the exemption would swallow the
        gate."""
        d = tree(**{"pkg__a_test.go": (
            "package pkg\n"
            "// see TestNoSuchPrefixAtAll for this\n"
            "func TestUnrelated(t *testing.T) {}\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual([n for _, _, n in fab], ["TestNoSuchPrefixAtAll"])
        finally:
            shutil.rmtree(d)


class DeclarationOracleTests(unittest.TestCase):
    """The oracle asks 'was this ever DECLARED', not 'did this string ever
    appear'. That distinction is the gate: a fabricated citation which has been
    committed once makes `-S<name>` return a commit — the one that added the
    false sentence — so an -S oracle calls it historical and passes. Measured
    2026-09-04: three of this branch's four fabrications had exactly that shape.
    """

    def pat(self, name):
        return cc.declaration_pattern(name)

    def test_the_pattern_matches_every_declaration_shape(self):
        import re
        for line, name in (
            ("func TestThing(t *testing.T) {", "TestThing"),
            ("func (s *Server) handleThing(w http.ResponseWriter) {", "handleThing"),
            ("type PieceThing struct{}", "PieceThing"),
            ("const MaxThingCount = 3600", "MaxThingCount"),
            ("var ErrThingGone = errors.New(\"x\")", "ErrThingGone"),
        ):
            self.assertRegex(line, self.pat(name), f"{name} should match {line!r}")

    def test_the_pattern_does_not_match_a_mere_mention(self):
        """The whole point. A comment citing the name is not a declaration, and
        if this passes the gate is answering the wrong question."""
        import re
        for line in (
            "// see TestThing for the precedent",
            "\tTestThing(t)",
            "// handleThing used to live here",
        ):
            self.assertNotRegex(line, self.pat("TestThing") + "|" + self.pat("handleThing"))

    def test_a_prefix_of_a_real_name_is_not_a_declaration_of_it(self):
        """Without the trailing character class, `func LoadPackTile` would read
        as declaring `LoadPack`, and every prefix of a real name would pass."""
        self.assertNotRegex("func LoadPackTile() {}", self.pat("LoadPack"))
        self.assertRegex("func LoadPack(dir string) {}", self.pat("LoadPack"))

    def test_a_missing_git_says_known_rather_than_inventing_findings(self):
        """A gate that cannot consult its oracle must not report fabrications:
        a false PASS is recoverable, a false FAIL trains people to ignore it."""
        d = tempfile.mkdtemp()  # not a git repository
        try:
            self.assertTrue(cc.git_knows("TotallyMadeUpNameHere", d))
        finally:
            shutil.rmtree(d)


class FalsePositiveTests(unittest.TestCase):
    """The three classes the gate's first real run produced, all of them correct
    prose. A gate whose first run prints four false findings gets switched off,
    so each is closed and each closure is pinned."""

    def test_a_typescript_declaration_counts(self):
        """Go comments cite the client constantly — tools/genmappack/std_pack.go
        names pack-assets.ts's loadStandardPackImages. The TS tree is never
        scanned for prose, but it must be scanned for declarations."""
        d = tree(**{
            "client__src__view__pack-assets.ts":
                "export async function loadStandardPackImages() {}\n",
            "tools__x.go":
                "package tools\n// pack-assets.ts's loadStandardPackImages iterates.\nfunc F() {}\n",
        })
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [], "a name declared in TypeScript is declared")
        finally:
            shutil.rmtree(d)

    def test_generated_go_declarations_count_even_though_it_is_not_scanned(self):
        d = tree(**{
            "contract__gen__go__v1__commands.pb.go":
                "package v1\ntype RemoveTokenThing struct{}\n",
            "cmd__x.go":
                "package cmd\n// see RemoveTokenThing in the contract\nfunc F() {}\n",
        })
        try:
            paths, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertNotIn("contract/gen", " ".join(str(p) for p in paths))
            self.assertEqual(fab, [], "generated declarations are still declarations")
        finally:
            shutil.rmtree(d)

    def test_a_wire_name_is_its_go_declarations_lowercased_twin(self):
        """A protobuf field generates as AnchorFromSeq and travels as
        anchorFromSeq; prose cites the wire form. Both are one thing."""
        d = tree(**{"pkg__a.go": (
            "package pkg\n"
            "// the generic dispatch decodes anchorFromSeq and anchorToSeq\n"
            "type Q struct {\n"
            "\tAnchorFromSeq int64 `protobuf:\"varint,3,opt,name=anchor_from_seq,"
            "json=anchorFromSeq,proto3\"`\n"
            "\tAnchorToSeq int64 `protobuf:\"varint,4,opt,name=anchor_to_seq,"
            "json=anchorToSeq,proto3\"`\n"
            "}\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)

    def test_a_dependency_symbol_is_resolvable_because_the_code_calls_it(self):
        """net/http declares ListenAndServe, cobra declares MarkFlagRequired.
        Neither is ours and both are findable, and a list of foreign words does
        not scale — the code that CALLS them is the evidence."""
        d = tree(**{"cmd__serve.go": (
            "package cmd\n"
            "// srv.ListenAndServe() with nothing observing Done().\n"
            "func run() { go func() { ch <- srv.ListenAndServe() }() }\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)

    def test_a_grouped_const_and_a_struct_field_are_resolvable(self):
        """Two shapes no line-anchored declaration regex sees, and both were in
        the 157 findings the declaration-only version produced."""
        d = tree(**{"pkg__a.go": (
            "package pkg\n"
            "// minDiceCount/maxDiceCount bound it; writeFrameMu guards the write.\n"
            "const (\n\tminDiceCount = 1\n\tmaxDiceCount = 100\n)\n"
            "type conn struct {\n\twriteFrameMu sync.Mutex\n}\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)

    def test_a_name_that_lives_only_in_prose_is_still_a_finding(self):
        """The whole point of widening `known`: it must not swallow the defect.
        A fabricated citation appears in a comment AND NOWHERE ELSE."""
        d = tree(**{"pkg__a.go": (
            "package pkg\n"
            "// see TestNothingCallsThisEver and handleImaginaryThing\n"
            "func F() {}\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(sorted(n for _, _, n in fab),
                             ["TestNothingCallsThisEver", "handleImaginaryThing"])
        finally:
            shutil.rmtree(d)

    def test_a_yaml_key_a_go_comment_cites_is_resolvable(self):
        """.go-arch-lint.yml's mayDependOn is cited by Go comments and is a real
        thing a reader can open."""
        d = tree(**{
            ".go-arch-lint.yml": "components:\n  x: { mayDependOn: [y] }\n",
            "pkg__a.go": "package pkg\n// identity is self-only (mayDependOn: [identity])\nfunc F() {}\n",
        })
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)

    def test_the_hatch_adjudicates_a_deliberate_counter_example(self):
        """Prose that says "X, not Y" names a Y that must NOT exist. The gate
        cannot tell that from a fabrication, so the pressure lands on a line
        carrying a reason — check-doc-owner.py's own discipline."""
        body = ("package pkg\n"
                "// commands are imperative-named (RemoveTokenThing, not "
                "RemoveTokenRequestThing)\n"
                "func F() {}\n")
        d = tree(**{"pkg__a.go": body})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual([n for _, _, n in fab],
                             ["RemoveTokenRequestThing", "RemoveTokenThing"])
        finally:
            shutil.rmtree(d)

        hatched = body.replace("func F() {}",
                               "//citations:ok the point is that it does not exist\nfunc F() {}")
        hatched = hatched.replace(
            "// commands are imperative-named",
            "//citations:ok the point is that it does not exist\n// commands are imperative-named")
        d2 = tree(**{"pkg__a.go": hatched})
        try:
            _, fab2, _ = cc.scan(d2, oracle=ORACLE_BLIND)
            self.assertEqual([n for _, _, n in fab2], [],
                             "the hatch must silence the line it sits on")
        finally:
            shutil.rmtree(d2)


class ScopeTests(unittest.TestCase):
    def test_generated_and_vendored_trees_are_skipped(self):
        d = tree(**{"contract__gen__go__x.go": (
            "package gen\n// TestNeverWasAnything is cited here\nfunc F() {}\n")})
        try:
            paths, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(paths, [])
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)

    def test_a_declaration_in_any_file_counts_everywhere(self):
        """Citations cross package boundaries constantly — internal/mapdef cites
        internal/artlib's tests — so `known` is the whole tree, not one file."""
        d = tree(**{
            "a__x_test.go": "package a\nfunc TestOverThere(t *testing.T) {}\n",
            "b__y.go": "package b\n// mirrors TestOverThere in package a\nfunc F() {}\n",
        })
        try:
            _, fab, hist = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
            self.assertEqual(hist, [])
        finally:
            shutil.rmtree(d)

    def test_types_consts_and_vars_count_as_declarations(self):
        d = tree(**{"pkg__a.go": (
            "package pkg\n"
            "// PieceOfArt, ErrArtDirUnreadable and MaxWireTilesCount are all real.\n"
            "type PieceOfArt struct{}\n"
            "var ErrArtDirUnreadable = errors.New(\"x\")\n"
            "const MaxWireTilesCount = 3600\n")})
        try:
            _, fab, _ = cc.scan(d, oracle=ORACLE_BLIND)
            self.assertEqual(fab, [])
        finally:
            shutil.rmtree(d)


class ExitCodeTests(unittest.TestCase):
    def run_main(self, root, *flags):
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = cc.main(["check-citations.py", root, *flags])
        return code, out.getvalue(), err.getvalue()

    def test_an_empty_tree_is_exit_2_not_a_pass(self):
        """A clean run over nothing is not a pass — the same guard
        check-doc-owner.py makes, for the same reason."""
        d = tempfile.mkdtemp()
        try:
            code, _, err = self.run_main(d)
            self.assertEqual(code, 2)
            self.assertIn("nothing was scanned", err)
        finally:
            shutil.rmtree(d)

    def test_a_fabrication_exits_1_and_a_clean_tree_exits_0(self):
        """main's two ordinary answers, driven through scan's real oracle
        parameter by monkeypatching it — main does not take one, deliberately:
        the injection seam belongs to scan, and main is where the real git
        oracle is chosen."""
        real = cc.git_knows
        d = tree(**{"pkg__a_test.go": (
            "package pkg\n"
            "// see TestNothingHasEverBeenCalledThis\n"
            "func TestReal(t *testing.T) {}\n")})
        try:
            cc.git_knows = ORACLE_BLIND
            code, _, err = self.run_main(d)
            self.assertEqual(code, 1)
            self.assertIn("TestNothingHasEverBeenCalledThis", err)
            self.assertIn("NEVER declared", err)

            cc.git_knows = ORACLE_OMNISCIENT
            code, out, _ = self.run_main(d)
            self.assertEqual(code, 0, "a name history knows is not fatal")
            self.assertIn("every cited name has existed", out)
        finally:
            cc.git_knows = real
            shutil.rmtree(d)

    def test_show_historical_lists_them_without_failing(self):
        real = cc.git_knows
        d = tree(**{"pkg__a.go": "package pkg\n// ErrLongGoneThing was here.\nfunc F() {}\n"})
        try:
            cc.git_knows = ORACLE_OMNISCIENT
            code, out, _ = self.run_main(d, "--show-historical")
            self.assertEqual(code, 0)
            self.assertIn("ErrLongGoneThing", out)
            self.assertIn("(historical)", out)
        finally:
            cc.git_knows = real
            shutil.rmtree(d)


if __name__ == "__main__":
    unittest.main(verbosity=2)
