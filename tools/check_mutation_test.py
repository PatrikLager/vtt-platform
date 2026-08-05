#!/usr/bin/env python3
"""Boundary tests for check-mutation.py.

This script is a gate, and the standing lesson from the coverage gate's five
review rounds is that gates ship with "passes when it should fail" holes. The
tests below inject a fake gremlins runner, so they assert on EXIT CODES and
which mutants are reported — the boundary — without running real mutation
testing (which takes ~111s).

Run: python3 tools/check_mutation_test.py
"""

import importlib.util
import io
import os
import tempfile
import unittest

_spec = importlib.util.spec_from_file_location(
    "check_mutation", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-mutation.py"))
cm = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cm)

def gremlins_output(*survivors, killed=9, timed_out=0, timed_out_locs=()):
    """Fake a gremlins run: survivors are (mutator, location) pairs."""
    lines = ["      KILLED CONDITIONALS_NEGATION at x.go:1:1"]
    lines += [f"       LIVED {m} at {loc}" for m, loc in survivors]
    lines += [f"   TIMED OUT {m} at {loc}" for m, loc in timed_out_locs]
    return "\n".join(lines) + (
        f"\nMutation testing completed in 1 seconds\n"
        f"Killed: {killed}, Lived: {len(survivors)}, Not covered: 0\n"
        f"Timed out: {timed_out}, Not viable: 0, Skipped: 0\n"
        f"Test efficacy: 100.00%\n")


def runner_for(mapping):
    """A fake runner returning canned output per package."""
    return lambda pkg: mapping.get(pkg, gremlins_output())


def equivalents(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


class MutationGateTest(unittest.TestCase):
    def gate(self, eq_path, mapping, packages=("./p/",)):
        out, err = io.StringIO(), io.StringIO()
        # A hermetic root, NOT the default ".". run()'s pre-flight guards walk
        # the tree they are given, so leaving this at the process cwd makes
        # every test below depend on what happens to sit in it — a stray
        # symlink anywhere under the working directory short-circuits run()
        # and reds all of them with a diagnosis from the wrong subsystem.
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq_path, list(packages), runner_for(mapping),
                          out=out, err=err, root=root)
        return code, out.getvalue(), err.getvalue()

    # --- the property the gate exists for ---

    def test_unadjudicated_survivor_fails(self):
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, _, err = self.gate(equivalents(""), m)
        self.assertEqual(code, 1)
        self.assertIn("a.go:10:5", err)
        self.assertIn("not adjudicated", err)

    def test_adjudicated_survivor_passes(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    assigns a value already held\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, out, _ = self.gate(eq, m)
        self.assertEqual(code, 0)
        self.assertIn("zero unadjudicated survivors", out)

    def test_no_survivors_passes(self):
        eq = equivalents("")
        code, out, _ = self.gate(eq, {"./p/": gremlins_output()})
        self.assertEqual(code, 0)
        self.assertIn("zero unadjudicated survivors", out)

    def test_adjudication_is_matched_on_all_three_fields(self):
        """A different mutator or location at the same spot is NOT pre-approved."""
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        for survivor in [("ARITHMETIC_BASE", "a.go:10:5"),     # same place, other mutator
                         ("CONDITIONALS_BOUNDARY", "a.go:11:5")]:  # same mutator, other line
            with self.subTest(survivor):
                code, _, err = self.gate(eq, {"./p/": gremlins_output(survivor)})
                self.assertEqual(code, 1)
                self.assertIn("not adjudicated", err)

    def test_survivor_in_a_different_package_is_not_pre_approved(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./q/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, _, err = self.gate(eq, m, packages=("./q/",))
        self.assertEqual(code, 1)
        self.assertIn("not adjudicated", err)

    # --- a stale entry must not silently pre-approve a future survivor ---

    def test_stale_entry_fails(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        code, _, err = self.gate(eq, {"./p/": gremlins_output()})
        self.assertEqual(code, 1)
        self.assertIn("no longer survives", err)

    # --- an equivalence claim without a stated reason is not a claim ---

    def test_entry_without_a_reason_is_fatal(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n")
        code, _, err = self.gate(eq, {"./p/": gremlins_output()})
        self.assertEqual(code, 1)
        self.assertIn("no reason line", err)

    def test_malformed_entry_is_fatal(self):
        eq = equivalents("p  a.go:10:5\n    reason\n")
        code, _, err = self.gate(eq, {"./p/": gremlins_output()})
        self.assertEqual(code, 1)
        self.assertIn("want", err)

    def test_missing_equivalents_file_is_fatal(self):
        code, _, err = self.gate("/nonexistent/equivalents.txt", {"./p/": gremlins_output()})
        self.assertEqual(code, 1)

    # --- a run dominated by timeouts is a broken measurement, not a pass ---

    def test_timeout_dominated_run_is_fatal(self):
        """Timed-out mutants are never evaluated, and gremlins' efficacy ignores them.

        Regression, and the sharpest self-inflicted one: adding two
        goroutine-waiting tests to internal/mcp turned a clean run (57 killed
        / 8 lived / 0 timed out) into 6 killed / 0 lived / 58 TIMED OUT — while
        gremlins still printed "Test efficacy: 100.00%", because efficacy is
        killed/(killed+lived). Parsing only LIVED lines passed it. 112 of 118
        mutants went unevaluated and the gate said clean.
        """
        m = {"./p/": gremlins_output(killed=6, timed_out=58)}   # 91%
        code, _, err = self.gate(equivalents(""), m)
        self.assertEqual(code, 1)
        self.assertIn("TIMED OUT", err)
        self.assertIn("broken measurement", err)

    def test_minority_timeouts_count_as_detections(self):
        """A mutant that HANGS the suite was detected — the suite did not pass.

        internal/gateway is the real case: 6 of 33 (18%) time out, all
        CONDITIONALS_NEGATION in the WebSocket connection lifecycle, where a
        broken mutant hangs a real socket rather than failing fast. Patrik's
        ruling: count them as kills.
        """
        m = {"./p/": gremlins_output(killed=27, timed_out=6)}   # 18%
        code, _, _ = self.gate(equivalents(""), m)
        self.assertEqual(code, 0)

    def test_the_cap_sits_between_the_two_observed_cases(self):
        """49% passes, 51% fails — calibrated against real runs, not taste.

        Measured: mcp-broken 91%, gateway 18%, mcp-fixed 2%. The cap has room
        on both sides rather than sitting next to either.
        """
        just_under = {"./p/": gremlins_output(killed=51, timed_out=49)}
        self.assertEqual(self.gate(equivalents(""), just_under)[0], 0)
        just_over = {"./p/": gremlins_output(killed=49, timed_out=51)}
        self.assertEqual(self.gate(equivalents(""), just_over)[0], 1)

    def test_timed_out_mutants_are_named_in_the_output(self):
        """A timed-out mutant can be a GENUINE SURVIVOR, so it must not vanish.

        Review counterexample: a lenient assertion over a value whose upper
        bound the mutation removes (`if d > ceiling { d = ceiling }` then
        sleep). gremlins reports TIMED OUT; applying that mutation by hand the
        suite PASSES in 60s. The mutant survives and this gate scores it as
        detected — so the minimum obligation is that it appears in the report.
        """
        m = {"./p/": gremlins_output(killed=9, timed_out=1,
                                     timed_out_locs=(("CONDITIONALS_NEGATION", "slow.go:9:7"),))}
        code, out, _ = self.gate(equivalents(""), m)
        self.assertEqual(code, 0, "per the ruling a timeout is not itself a failure")
        self.assertIn("slow.go:9:7", out)
        self.assertIn("NOT measured", out)

    def test_unparseable_summary_counts_are_fatal(self):
        """No `Timed out:` line means the timeout check cannot run.

        Failing open would re-create the blind spot this gate was built after:
        a grep that never read that line reported two packages as "100%
        efficacy" while a quarter to a majority went unevaluated.
        """
        broken = ("      KILLED CONDITIONALS_NEGATION at x.go:1:1\n"
                  "Mutation testing completed in 1 seconds\n"
                  "TimedOut: 58\n")   # renamed field
        code, _, err = self.gate(equivalents(""), {"./p/": broken})
        self.assertEqual(code, 1)
        self.assertIn("could not parse", err)

    def test_exactly_half_timed_out_is_accepted(self):
        """The boundary itself, which a mutation audit found unpinned.

        `> MAX_TIMEOUT_FRACTION` means 50% passes and 51% fails. Mutating `>`
        to `>=` survived because no test sat on the line. Recorded here so the
        decision is the code's, not the reader's.
        """
        m = {"./p/": gremlins_output(killed=50, timed_out=50)}
        self.assertEqual(self.gate(equivalents(""), m)[0], 0)

    # --- a gremlins run that did not complete must not read as success ---

    def test_incomplete_gremlins_run_is_fatal(self):
        """No summary line means the run died; zero parsed survivors would
        otherwise look identical to a clean package."""
        code, _, err = self.gate(equivalents(""), {"./p/": "panic: something broke"})
        self.assertEqual(code, 1)
        self.assertIn("did not complete", err)

    # --- parsing the real format ---

    def test_parses_gremlins_lived_lines(self):
        real = ("       LIVED CONDITIONALS_BOUNDARY at apply.go:141:15\n"
                "      KILLED CONDITIONALS_NEGATION at apply.go:100:44\n"
                "       LIVED CONDITIONALS_BOUNDARY at apply.go:144:30\n")
        got = cm.parse_survivors(real)
        self.assertEqual(got, [("apply.go:141:15", "CONDITIONALS_BOUNDARY"),
                               ("apply.go:144:30", "CONDITIONALS_BOUNDARY")])

    def test_killed_lines_are_never_counted_as_survivors(self):
        self.assertEqual(cm.parse_survivors("      KILLED CONDITIONALS_BOUNDARY at a.go:1:1\n"), [])

    def test_multiple_packages_are_all_checked(self):
        eq = equivalents("")
        m = {"./p/": gremlins_output(), "./q/": gremlins_output(("ARITHMETIC_BASE", "b.go:2:2"))}
        code, _, err = self.gate(eq, m, packages=("./p/", "./q/"))
        self.assertEqual(code, 1, "a survivor in the SECOND package must still fail the gate")
        self.assertIn("b.go:2:2", err)


def _pkg_dir(root, rel, clause, with_test=True):
    """Create <root>/<rel>/ containing a .go file declaring `package <clause>`."""
    d = os.path.join(root, rel)
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "thing.go"), "w") as fh:
        fh.write(f"package {clause}\n\nfunc F() int {{ return 1 }}\n")
    if with_test:
        with open(os.path.join(d, "thing_test.go"), "w") as fh:
            fh.write(f"package {clause}\n")
    return d


class PackageResolutionTest(unittest.TestCase):
    """A package gremlins cannot resolve reports EVERY mutant as killed.

    gremlins picks a mutant's test target by walking up from the mutated file
    for a directory whose NAME equals the declared package name. A `package
    main` in a directory not named `main` has no such ancestor, so it falls
    back to the bare MODULE PATH — `go test github.com/owner/repo` — which
    does not resolve. That exits 1, and exit 1 is exactly what gremlins scores
    as KILLED. Every mutant becomes a false kill in ~11ms and the gate prints
    a clean measurement for a run that never executed a single test.

    Found 2026-08-04 by review, in a change that was ADDING cmd/vtt to the
    gate on the strength of "91 killed, 0 lived, ~9s". `go test ./cmd/vtt/`
    alone takes 29s, and a PROVABLY EQUIVALENT mutant (state_dump.go:153,
    `>` -> `>=`, reassigning the same value) was reported KILLED. tools/toolgen
    had the same shape and had been in the gated set all along.

    Verifying a KILLABLE mutant cannot catch this — a constant "everything
    dies" verdict passes that check too. Hence a structural guard rather than
    a sampled one.
    """

    def test_a_package_whose_directory_matches_its_clause_is_resolvable(self):
        with tempfile.TemporaryDirectory() as root:
            _pkg_dir(root, "internal/identity", "identity")
            self.assertEqual(cm.unresolvable_packages(["./internal/identity/"], root=root), [])

    def test_package_main_in_a_mismatched_directory_is_reported(self):
        with tempfile.TemporaryDirectory() as root:
            _pkg_dir(root, "cmd/vtt", "main")
            bad = cm.unresolvable_packages(["./cmd/vtt/"], root=root)
            self.assertEqual(len(bad), 1, "package main in a dir not named 'main' is unresolvable")
            self.assertEqual(bad[0][0], "./cmd/vtt/")
            self.assertEqual(bad[0][1], "main", "the DECLARED package name")
            self.assertEqual(bad[0][2], "vtt", "the DIRECTORY name that fails to match it")

    def test_any_mismatch_counts_not_only_package_main(self):
        # The failure is name inequality, not the word "main". A package
        # deliberately named differently from its directory breaks identically.
        with tempfile.TemporaryDirectory() as root:
            _pkg_dir(root, "internal/rules", "ruleset")
            self.assertEqual(len(cm.unresolvable_packages(["./internal/rules/"], root=root)), 1)

    def test_the_gate_fails_and_explains_rather_than_reporting_a_clean_run(self):
        with tempfile.TemporaryDirectory() as root:
            _pkg_dir(root, "cmd/vtt", "main")
            eq = equivalents("")
            buf_out, buf_err = io.StringIO(), io.StringIO()
            code = cm.run(eq, packages=["./cmd/vtt/"], runner=runner_for({}),
                          out=buf_out, err=buf_err, root=root)
            os.unlink(eq)
            self.assertEqual(code, 1, "an unmeasurable package must FAIL, never report ok")
            self.assertIn("cmd/vtt", buf_err.getvalue())
            self.assertNotIn("ok  cmd/vtt", buf_out.getvalue(),
                             "the gate must not print a clean line for a package it cannot measure")

    def test_test_only_package_clauses_are_ignored(self):
        # An external test package (foo_test) sits beside the real one and must
        # not be mistaken for the declaration.
        with tempfile.TemporaryDirectory() as root:
            d = _pkg_dir(root, "internal/store", "store")
            with open(os.path.join(d, "zz_external_test.go"), "w") as fh:
                fh.write("package store_test\n")
            self.assertEqual(cm.unresolvable_packages(["./internal/store/"], root=root), [])

    def test_every_gated_package_in_the_real_tree_is_resolvable(self):
        # The guard pointed at the actual PACKAGES list. This is the assertion
        # that would have stopped cmd/vtt and toolgen being gated at all.
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        self.assertEqual(cm.unresolvable_packages(cm.PACKAGES, root=repo), [],
                         "every gated package must be one gremlins can actually resolve")


class SubpackageRecursionTest(unittest.TestCase):
    """A gated parent must not re-measure a gated child.

    `gremlins unleash ./internal/rules/` RECURSES: it mutates files under
    internal/rules/conformance/ too, and reports them relative to the package
    it was pointed at — `conformance/conformance.go:207:11`. When BOTH are in
    PACKAGES that is wrong twice over:

      1. The same mutant is measured under two keys. An adjudication written
         for ('internal/rules/conformance', 'conformance.go:207:11') does not
         match ('internal/rules', 'conformance/conformance.go:207:11'), so the
         gate reports an already-excused survivor as unadjudicated — and the
         only way to green it would be a DUPLICATE entry that then drifts.
      2. It is measured twice, costing the runtime twice. internal/adventure's
         run carried 23 of its conformance child's mutants; internal/rules'
         carried 60.

    Found 2026-08-04 while measuring internal/rules: two survivors turned up in
    a file that is not in internal/rules. The gate was green and honest
    throughout — internal/adventure/conformance simply had no survivors to
    double-report — which is exactly why nothing pointed at it.

    Excluding via gremlins' own --exclude-files rather than filtering results
    here is deliberate: check-mutation.py's header records that 8 of 9 defects
    across five review rounds lived in hand-rolled enforcement code. Filtering
    survivors by path would also silently drop mutants from subpackages that
    are NOT separately gated, converting a visible gap into an invisible one.
    """

    def test_a_gated_child_is_excluded_from_its_parents_run(self):
        args = cm.gremlins_args("./internal/rules/",
                                ["./internal/rules/", "./internal/rules/conformance/"])
        self.assertIn("--exclude-files", args)
        self.assertIn("^conformance/", args, "the child's path RELATIVE to the parent")

    def test_an_ungated_subdirectory_is_left_alone(self):
        # Not gated separately => the parent is the only thing measuring it,
        # so excluding it would drop those mutants silently.
        args = cm.gremlins_args("./internal/rules/", ["./internal/rules/"])
        self.assertNotIn("--exclude-files", args)

    def test_a_sibling_is_not_mistaken_for_a_child(self):
        args = cm.gremlins_args("./internal/rules/",
                                ["./internal/rules/", "./internal/rulesets/"])
        self.assertNotIn("--exclude-files", args,
                         "internal/rulesets is NOT under internal/rules despite the prefix")

    def test_a_parent_is_not_excluded_from_its_childs_run(self):
        args = cm.gremlins_args("./internal/rules/conformance/",
                                ["./internal/rules/", "./internal/rules/conformance/"])
        self.assertNotIn("--exclude-files", args)

    def test_every_gated_child_is_excluded_not_just_the_first(self):
        args = cm.gremlins_args("./a/", ["./a/", "./a/b/", "./a/c/"])
        self.assertIn("^b/", args)
        self.assertIn("^c/", args)

    def test_the_real_packages_list_excludes_its_real_children(self):
        # DERIVED from PACKAGES, not hardcoded: a hardcoded
        # ("./internal/adventure/", "^conformance/") pair keeps PASSING if
        # internal/adventure is ever removed from PACKAGES, so the one
        # assertion that would have caught this bug goes silently vacuous
        # exactly when the list changes under it.
        norm = [p.strip("./").rstrip("/") for p in cm.PACKAGES]
        pairs = [(p, c) for p in norm for c in norm if c.startswith(p + "/")]
        self.assertTrue(pairs, "no gated parent/child pair left in PACKAGES — this test is vacuous")
        for parent, child in pairs:
            args = cm.gremlins_args(f"./{parent}/", cm.PACKAGES)
            self.assertIn(f"^{child[len(parent) + 1:]}/", args,
                          f"{parent} must exclude its gated child {child}")


class DroppedSymlinkTest(unittest.TestCase):
    """gremlins SILENTLY drops symlinks, and a test that needs one then fails
    for a reason that has nothing to do with the mutation.

    gremlins copies the module before mutating it. That copy is a filepath.Walk
    whose handler `copyPath` (workdir.go:142) switches on `mode.IsDir()` and
    `mode.IsRegular()` — a symlink is NEITHER, so it falls through the switch
    and is skipped without an error. In the copy the path simply is not there.

    Any test that reads it then fails in the copy, ALWAYS, for every mutant,
    exiting 1 — and exit 1 is what gremlins scores as KILLED, so the package
    reports 100% efficacy having detected nothing. This is the same
    constant-verdict failure
    as PackageResolutionTest above, reached by a different route, and it is
    equally invisible: the output is indistinguishable from a well-tested
    package.

    Found 2026-08-05 while testing whether --integration could gate cmd/vtt.
    It could not, and neither could the renamed-worktree measurement that
    tools/mutation-scope.md had published as "75 killed, no genuine survivors
    among the 77 evaluated". Both were this artifact:
    scenarios/testdata/dnd45e-minimal-adventures/goblin-ambush is a symlink,
    cmd/vtt's scenario tests boot a server against that directory, and in the
    copy it is empty -- "adventures dir ... contains no adventures".

    The allowlist takes a REASON per entry, and the reason names which packages
    the symlink makes unmeasurable. That is the whole point of recording it:
    a bare "we know about this one" would let the next person gate a package
    the entry already forbids.
    """

    def _tree(self, root, link_at, target="target"):
        os.makedirs(os.path.join(root, "adventures", target), exist_ok=True)
        link = os.path.join(root, link_at)
        os.makedirs(os.path.dirname(link), exist_ok=True)
        os.symlink(os.path.join(root, "adventures", target), link)
        return link

    def test_a_symlink_outside_the_allowlist_is_reported(self):
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "scenarios/testdata/fixture/goblin")
            self.assertEqual(cm.dropped_symlinks(root=root, allowed={}),
                             ["scenarios/testdata/fixture/goblin"])

    def test_an_allowlisted_symlink_is_accepted(self):
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "scenarios/testdata/fixture/goblin")
            allowed = {"scenarios/testdata/fixture/goblin": "recorded, with its consequence"}
            self.assertEqual(cm.dropped_symlinks(root=root, allowed=allowed), [])

    def test_a_symlink_to_a_DIRECTORY_is_reported(self):
        # The real case, and the one os.walk is most likely to hide: a
        # symlinked directory lands in dirnames, not filenames.
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "scenarios/link-to-dir")
            self.assertTrue(os.path.isdir(os.path.join(root, "scenarios/link-to-dir")))
            self.assertEqual(cm.dropped_symlinks(root=root, allowed={}),
                             ["scenarios/link-to-dir"])

    def test_a_regular_file_is_not_reported(self):
        with tempfile.TemporaryDirectory() as root:
            os.makedirs(os.path.join(root, "pkg"))
            with open(os.path.join(root, "pkg", "real.go"), "w") as fh:
                fh.write("package pkg\n")
            self.assertEqual(cm.dropped_symlinks(root=root, allowed={}), [])

    def test_vendored_and_git_trees_are_not_walked(self):
        # node_modules is full of symlinks and is not part of the Go module;
        # walking it would make the guard both slow and permanently red.
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "node_modules/.bin/tsc")
            self._tree(root, ".git/annex/link")
            self.assertEqual(cm.dropped_symlinks(root=root, allowed={}), [])

    def test_a_symlink_NAMED_like_a_vendored_dir_is_still_reported(self):
        # The pruning must not shadow the check: a symlink named node_modules
        # is a dropped symlink like any other, and skipping it because of its
        # name is how a guard grows a hole shaped like its own exclusion list.
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "node_modules")
            self.assertEqual(cm.dropped_symlinks(root=root, allowed={}), ["node_modules"])

    def test_an_allowlist_entry_without_a_reason_is_fatal(self):
        # Same rule as tools/mutation-equivalents.txt: an excuse with no
        # argument is how a real gap becomes a permanent one in writing.
        with tempfile.TemporaryDirectory() as root:
            self._tree(root, "scenarios/x/link")
            with self.assertRaises(cm.EquivalentsError):
                cm.dropped_symlinks(root=root, allowed={"scenarios/x/link": "   "})

    def test_the_gate_fails_and_names_the_symlink_rather_than_measuring(self):
        with tempfile.TemporaryDirectory() as root:
            _pkg_dir(root, "internal/thing", "thing")
            self._tree(root, "scenarios/x/link")
            eq = equivalents("")
            buf_out, buf_err = io.StringIO(), io.StringIO()
            code = cm.run(eq, packages=["./internal/thing/"], runner=runner_for({}),
                          out=buf_out, err=buf_err, root=root)
            os.unlink(eq)
            self.assertEqual(code, 1, "an unrecorded symlink must FAIL the gate")
            self.assertIn("scenarios/x/link", buf_err.getvalue())
            self.assertNotIn("zero unadjudicated survivors", buf_out.getvalue(),
                             "the gate must not report a clean run it cannot vouch for")

    def test_the_real_tree_carries_no_unrecorded_symlink(self):
        # The live assertion. If someone adds a symlink, this fails until they
        # record which packages it makes unmeasurable.
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        self.assertEqual(cm.dropped_symlinks(root=repo), [],
                         "a symlink gremlins drops must be recorded with its consequence")

    def test_the_allowlist_is_not_vacuous(self):
        # Guards against the entry being deleted along with the symlink and the
        # test above quietly becoming an assertion about nothing.
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        found = cm.dropped_symlinks(root=repo, allowed={})
        self.assertEqual(sorted(found), sorted(cm.ALLOWED_SYMLINKS),
                         "ALLOWED_SYMLINKS must list exactly the symlinks that exist — if one "
                         "was removed, delete its entry too, or a prohibition nobody can satisfy "
                         "stays in writing")


if __name__ == "__main__":
    unittest.main(verbosity=2)
