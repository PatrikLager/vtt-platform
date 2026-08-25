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

def gremlins_output(*survivors, killed=9, timed_out=0, timed_out_locs=(), not_covered=0):
    """Fake a gremlins run: survivors are (mutator, location) pairs.

    THE LINES AND THE SUMMARY AGREE BY CONSTRUCTION, because real gremlins
    output does. Measured 2026-08-10 on internal/rules: 553 per-mutant lines
    against a summary of 498 killed + 16 lived + 38 not covered + 1 timed out,
    and NOT COVERED does get a line of its own.

    This used to emit ONE `KILLED` line beside a summary claiming nine, which
    is output gremlins cannot produce. A fixture that models impossible output
    is a lie of its own, and this one hid the tally check — the cheapest
    available cross-check on a run that silently lost 88 mutants to a full disk.
    """
    lines = [f"      KILLED CONDITIONALS_NEGATION at k{i}.go:1:1" for i in range(killed)]
    lines += [f"       LIVED {m} at {loc}" for m, loc in survivors]
    lines += [f"  NOT COVERED CONDITIONALS_NEGATION at n{i}.go:1:1" for i in range(not_covered)]
    # Named locations when given, otherwise as many anonymous ones as the
    # summary claims — either way the two halves match.
    locs = list(timed_out_locs) or [("CONDITIONALS_NEGATION", f"t{i}.go:1:1") for i in range(timed_out)]
    lines += [f"   TIMED OUT {m} at {loc}" for m, loc in locs]
    return "\n".join(lines) + (
        f"\nMutation testing completed in 1 seconds\n"
        f"Killed: {killed}, Lived: {len(survivors)}, Not covered: {not_covered}\n"
        f"Timed out: {len(locs)}, Not viable: 0, Skipped: 0\n"
        f"Test efficacy: 100.00%\n")


def runner_for(mapping):
    """A fake runner returning canned output per package."""
    return lambda pkg: mapping.get(pkg, gremlins_output())


def equivalents(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False)
    fh.write(text)
    fh.close()
    return fh.name


# The source every equivalents fixture below is keyed against.
#
# It has to be REAL source at REAL columns, because the gate now reads the tree
# at each key's file:line:col and rejects a key that no longer points at a
# mutant of the kind it names. A fixture of blank lines would make every test
# here fail with a position complaint instead of the thing it is about.
#
# COLUMNS ARE 1-BASED BYTE OFFSETS and the operator has to land on the one the
# key already spells, which is why these read `if a<b {` rather than the
# gofmt-ed `if a < b {`: the historic fixtures in this file say 10:5, and the
# column is the thing under test.
#
# The lines are DELIBERATELY DISTINCT where the tests rely on two keys being
# different claims (10 vs 11 vs 14), and DELIBERATELY IDENTICAL where they rely
# on a pairing being possible (30/40/50/60). That is the whole content-anchor
# idea in a fixture: two keys pair when the source says they are the same
# statement, and never merely because they share a file and a mutator.
GO_LINES = {
    10: "if a<b {",            # col 5 is `<`   -- the canonical adjudication
    11: "if c>d {",            # col 5 is `>`   -- one line away, DIFFERENT text
    14: "if e<f {",            # col 5 is `<`   -- same token, different text
    20: "if abcde<f {",        # col 9 is `<`
    30: "if dup<lim {",        # col 7 is `<`   -- four copies of ONE statement,
    40: "if dup<lim {",        # col 7 is `<`      so a move between them is
    50: "if dup<lim {",        # col 7 is `<`      indistinguishable by content
    60: "if dup<lim {",        # col 7 is `<`      and pairable by it
    70: "// a line comment",
    71: "/* a block comment */",
    72: " * a doc-comment continuation line",
    80: "n := a+b",            # col 7 is `+`
    90: "\tif tab<x {",        # col 8 is `<`, counting the TAB as one byte
}


def _go_file(path, clause):
    """Write GO_LINES out as a .go file, padded so the line numbers are real.

    The package clause has to MATCH THE DIRECTORY NAME or unresolvable_packages
    refuses the run before anything is measured — a real guard, and one that
    would otherwise red every test here with a diagnosis from the wrong
    subsystem now that these directories contain Go source at all.
    """
    os.makedirs(os.path.dirname(path), exist_ok=True)
    body = [f"package {clause}", "", "func F() {"]
    while len(body) < max(GO_LINES):
        body.append(GO_LINES.get(len(body) + 1, "\t_ = 0"))
    with open(path, "w") as fh:
        fh.write("\n".join(body) + "\n}\n")


def source_tree(root):
    """The two packages and two files every fixture key in this file names."""
    for pkg in ("p", "q"):
        for name in ("a.go", "b.go"):
            _go_file(os.path.join(root, pkg, name), pkg)
    return root


class MutationGateTest(unittest.TestCase):
    def gate(self, eq_path, mapping, packages=("./p/",)):
        out, err = io.StringIO(), io.StringIO()
        # A hermetic root, NOT the default ".". run()'s pre-flight guards walk
        # the tree they are given, so leaving this at the process cwd makes
        # every test below depend on what happens to sit in it — a stray
        # symlink anywhere under the working directory short-circuits run()
        # and reds all of them with a diagnosis from the wrong subsystem.
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            # free_bytes pinned for the same reason root is: without it every
            # test here reads the REAL disk, and on a machine below the floor
            # all of them fail with a diagnosis from the wrong subsystem — 26
            # unrelated assertion failures in place of the one clear message
            # the pre-flight exists to give. The disk tests pass it explicitly.
            code = cm.run(eq_path, list(packages), runner_for(mapping),
                          out=out, err=err, root=root, free_bytes=500 * 1024**3,
                          cache_bytes=0)
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

    def stale_line(self, err):
        """The one 'no longer survives' line, so a hint assertion is scoped to it.

        Asserting against the whole stream is how these went vacuous: a
        location named by the hint is ALSO named by its own "SURVIVED and is
        not adjudicated" line, so `assertIn(loc, err)` passes whether the hint
        mentions it or not.
        """
        lines = [l for l in err.splitlines() if "no longer survives" in l]
        self.assertEqual(len(lines), 1, f"want exactly one stale-entry line, got {lines}")
        return lines[0]

    # --- a stale entry beside a same-file survivor gets a HINT, not a claim ---
    #
    # The delete-the-adjudication reflex is the harm being addressed. "This
    # entry no longer survives, remove it" reads as an instruction, and when an
    # edit has merely SHIFTED the mutant, obeying it throws away reasoning and
    # leaves the survivor unexplained. Measured: that happened four times in one
    # day, across both gates.
    #
    # THIS IS THE RESIDUE, not the whole answer, and it stopped being the whole
    # answer when moved_entries landed — see MovedAdjudicationTest below. A
    # move the SOURCE can prove (same package, file, mutator, operator and line
    # text) is now reported as a move with its new key. What is left here is
    # the ordinary shift, where an insertion above leaves the entry's old line
    # holding somebody else's code and nothing in the tree can pair the two:
    # the Go key carries no replacement text, so "same file, same mutator" on
    # its own cannot tell a moved mutant from a different one a line away —
    # which is precisely what test_adjudication_is_matched_on_all_three_fields
    # above pins. For that residue the hint changes the MESSAGE and never the
    # verdict.

    def test_stale_entry_hints_at_a_same_file_survivor(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:14:5"))}
        code, _, err = self.gate(eq, m)

        self.assertEqual(code, 1)
        hint = self.stale_line(err)
        self.assertIn("a.go:14:5", hint)
        self.assertIn("RE-KEY", hint)
        # A HINT. It must say so, or the next reader takes it for a match and
        # re-keys an adjudication onto a mutant nobody judged.
        self.assertIn("hint, not a match", err)

    def test_the_hint_does_not_excuse_anything(self):
        """The verdict is unchanged: BOTH are still reported, and it still fails.

        This is the invariant the first attempt at this broke. Pairing the two
        as a move made the survivor stop being reported, which silently
        pre-approved a mutant nobody had judged — the exact failure the gate
        exists to prevent, reached by way of a convenience.
        """
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:14:5"))}
        code, _, err = self.gate(eq, m)

        self.assertEqual(code, 1)
        self.assertIn("not adjudicated", err)   # the survivor, still unexcused
        self.assertIn("no longer survives", err)  # the stale entry, still stale

    def test_no_hint_when_nothing_could_have_moved(self):
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        for survivor in [("ARITHMETIC_BASE", "a.go:14:5"),  # other mutator
                         ("CONDITIONALS_BOUNDARY", "b.go:14:5")]:  # other file
            with self.subTest(survivor):
                code, _, err = self.gate(eq, {"./p/": gremlins_output(survivor)})
                self.assertEqual(code, 1)
                self.assertNotIn("RE-KEY", err)

    def test_the_hint_names_every_candidate(self):
        """Never one. Picking a favourite is the guess this design refuses.

        Asserted against the HINT LINE, not the whole stream. Both locations
        appear in err regardless — each has its own "SURVIVED and is not
        adjudicated" line — so `assertIn(loc, err)` was satisfied with the hint
        entirely absent. Proven: naming a single candidate left the suite green.
        """
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:14:5"),
                                     ("CONDITIONALS_BOUNDARY", "a.go:20:9"))}
        code, _, err = self.gate(eq, m)
        self.assertEqual(code, 1)
        hint = self.stale_line(err)
        self.assertIn("a.go:14:5", hint)
        self.assertIn("a.go:20:9", hint)

    def test_no_hint_across_packages(self):
        """Same file, same mutator, DIFFERENT package is not a candidate.

        Without this, dropping the package check makes the gate point a RE-KEY
        at a mutant in another package entirely — and the rest of the suite
        stays green.
        """
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./q/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:14:5"))}
        code, _, err = self.gate(eq, m, packages=("./q/",))
        self.assertEqual(code, 1)
        self.assertNotIn("RE-KEY", err)

    def test_a_package_written_differently_is_diagnosed_as_such(self):
        """The same location under a differently-spelled package is not a move.

        read_equivalents takes the package verbatim while survivors carry the
        normalised name, so `./p/` in the file matches nothing and BOTH halves
        are reported — loudly, but with no clue that the two lines are the same
        mutant. That cost a real debugging session: the first attempt at
        pairing silently matched nothing for exactly this reason.
        """
        eq = equivalents("./p/  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, _, err = self.gate(eq, m)

        self.assertEqual(code, 1)
        hint = self.stale_line(err)
        # BOTH spellings, so the reader can see the difference rather than
        # being told there is one. The correction alone leaves them hunting.
        self.assertIn("'./p/'", hint)
        self.assertIn("'p'", hint)
        # Stated as a DEDUCTION, not hedged. If the spellings agreed the keys
        # would be equal and this entry would not be stale, so the mismatch is
        # certain — and hedging certain, actionable advice invites the reader
        # to ignore it.
        self.assertIn("A DEDUCTION", hint)
        # Still not excused: the entry does not apply, and the gate says so.
        self.assertIn("not adjudicated", err)

    # --- too little disk is refused BEFORE the run, not discovered after it ---
    #
    # The corruption guards below catch a spoiled run. This catches the cause
    # before any of it is spent: gremlins copies the tree per mutant into
    # $TMPDIR, and `task check:mutation` grows GOCACHE without bound — measured
    # 2026-08-10, one full gate run consumed 7.6 GB of free space and left
    # GOCACHE 4 GB larger, which is how it reached 82 GB and filled a 228 GB
    # volume. When it fills, Bash itself stops working, so the failure does not
    # even present as a gate failure.

    def test_too_little_disk_refuses_before_running_anything(self):
        ran = []

        def runner(pkg):
            ran.append(pkg)
            return gremlins_output()

        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/"], runner, out=out, err=err, root=root,
                          free_bytes=3 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 1)
        self.assertEqual(ran, [], "nothing may be measured on a disk this full")
        # The NUMBERS, both of them: an operator needs to know what they have
        # and what is wanted, not just that something is wrong.
        # "3" alone matched the TMPDIR PATH, which on this machine contains a
        # 3. Both numbers, spelled as the message spells them.
        self.assertIn("3.0 GiB free", err.getvalue())
        self.assertIn("16 GiB", err.getvalue())
        self.assertIn("go clean -cache", err.getvalue())

    def test_enough_disk_runs_normally(self):
        """The control. A floor that refuses every run gates nothing."""
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 0, err.getvalue())

    def test_the_floor_is_above_one_run_s_measured_cost(self):
        """A floor below what a run actually spends would let it start and die.

        7.6 GB is the measured cost of one full gate run. A floor at or under
        that is not a floor — it admits a run that cannot finish, which is the
        state this guard exists to prevent.
        """
        self.assertGreater(cm.MIN_FREE_BYTES, 8 * 1024**3)

    # --- the deadline every mutant is judged against ---
    #
    # gremlins derives it from the wall time of its OWN coverage run, which
    # omits -count=1 and is therefore eligible for Go's test result cache.
    # Measured on internal/gateway back to back: 9.286s then (cached) 0.27s —
    # a 41x collapse that drops the deadline BELOW the suite's own runtime, so
    # every mutant times out and the gate reports a broken measurement.

    def test_the_test_cache_is_cleared_before_any_package_is_measured(self):
        """ORDER, not just occurrence. Clearing it after the first package has
        run leaves that package judged against a collapsed deadline — which is
        the failure, not a smaller version of it."""
        events = []

        def prepare():
            events.append("clear")

        def runner(pkg):
            events.append(pkg)
            return gremlins_output()

        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/", "./q/"], runner, out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0, prepare=prepare)
        self.assertEqual(code, 0, err.getvalue())
        self.assertEqual(events, ["clear", "./p/", "./q/"],
                         "the cache must be cleared once, before the first package")

    def test_a_disk_too_full_to_run_does_not_bother_clearing_the_cache(self):
        """The floor refuses before anything is spent, and throwing away the
        test cache is a cost — the next ordinary `go test` pays for it. A guard
        that refuses AND charges you is worse than one that just refuses."""
        cleared = []
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                          free_bytes=3 * 1024**3, cache_bytes=0,
                          prepare=lambda: cleared.append(1))
        self.assertEqual(code, 1)
        self.assertEqual(cleared, [], "a refused run must not clear the cache")

    def test_the_real_clear_names_the_test_cache_not_the_build_cache(self):
        """The distinction the first diagnosis of this got wrong, and it decides
        the cost: `go clean -testcache` discards cached RESULTS and nothing has
        to be recompiled, while `go clean -cache` forces a full rebuild of the
        module — minutes, on every gate run, for no benefit."""
        self.assertEqual(cm.clear_test_cache_args(), ["go", "clean", "-testcache"])

    # --- 16 GiB free is a CLIFF; these pin the ramp in front of it ---
    #
    # The floor fails closed, but only once the volume is nearly gone, on a
    # machine that may by then be too full for Bash. The warning speaks while
    # there is still room to act, and never changes the exit code.

    def test_a_disk_heading_for_the_floor_warns_and_still_runs(self):
        """A WARNING, not a floor. Refusing here would be a new gate, and a gate
        change is its own reviewed decision (CLAUDE.md rule 2).

        `ran` is asserted, not just the exit code: a mutant that returned 0
        WITHOUT running anything would satisfy an exit-code-only test while
        reporting success over a gate that never executed.
        """
        ran = []

        def runner(pkg):
            ran.append(pkg)
            return gremlins_output()

        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/"], runner, out=out, err=err, root=root,
                          free_bytes=20 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 0, err.getvalue())
        self.assertEqual(ran, ["./p/"], "a warning must not stop the measurement")
        self.assertIn("20.0 GiB free", err.getvalue())
        self.assertIn("16 GiB floor", err.getvalue())

    def test_a_roomy_disk_says_nothing_at_all(self):
        """The control, and the assertion that matters most.

        assertEqual on the WHOLE stream, not assertNotIn on a chosen word:
        review made the warning unconditional and reworded it, and a
        `assertNotIn("GOCACHE", ...)` control SURVIVED. A clean run writes
        nothing to stderr, so the strongest form is also the simplest.
        """
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 0, err.getvalue())
        self.assertEqual(err.getvalue(), "")

    def test_exactly_at_the_warning_mark_is_silent(self):
        """The boundary. `<` mutated to `<=` survives unless a test sits on the
        line -- this file already carries test_exactly_half_timed_out_is_accepted,
        written for that exact reason one screen up."""
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                   free_bytes=cm.CACHE_WARN_FREE_BYTES, cache_bytes=0)
        self.assertEqual(err.getvalue(), "")

    def test_the_warning_says_what_cleaning_the_cache_would_recover(self):
        """The du half, and the only thing it is for: an operator deciding
        whether `go clean -cache` is worth it needs the number."""
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                   free_bytes=20 * 1024**3, cache_bytes=37 * 1024**3)
        self.assertIn("37.0 GiB", err.getvalue())
        self.assertIn("go clean -cache", err.getvalue())

    def test_an_unmeasurable_cache_still_warns_without_the_recover_clause(self):
        """Fail SOFT, and note WHICH way. Because the trigger is free space, a
        du that cannot answer costs one clause of the message -- not the
        message. Under the first design, which triggered on cache size, the same
        failure suppressed the warning entirely."""
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        real, cm.cache_size = cm.cache_size, lambda _: None
        try:
            with tempfile.TemporaryDirectory() as root:
                cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                       free_bytes=20 * 1024**3)
        finally:
            cm.cache_size = real
        self.assertIn("20.0 GiB free", err.getvalue())
        self.assertNotIn("go clean -cache", err.getvalue())

    def test_the_production_path_measures_the_cache_itself(self):
        """WIRING, not logic. Every other test here injects cache_bytes, so
        deleting the two lines that actually call cache_size(go_cache()) left
        the whole suite green and the feature dead in production. Review found
        that by deleting them."""
        eq = equivalents("")
        out, err = io.StringIO(), io.StringIO()
        real, cm.cache_size = cm.cache_size, lambda _: 41 * 1024**3
        try:
            with tempfile.TemporaryDirectory() as root:
                code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                              free_bytes=20 * 1024**3)
        finally:
            cm.cache_size = real
        self.assertEqual(code, 0, err.getvalue())
        self.assertIn("41.0 GiB", err.getvalue())

    def test_cache_size_measures_a_real_directory(self):
        """The measurement path, which no other test reaches -- the missing-dir
        test stops at the isdir guard. Review injected three mutants that all
        survived: dropping the *1024 (a 4 GiB cache reads as 4 MB), taking
        split()[-1] (parses the path, ValueError, None), and measuring $TMPDIR
        instead of the argument. A range, not equality: du reports allocated
        blocks, not bytes."""
        with tempfile.TemporaryDirectory() as d:
            with open(os.path.join(d, "payload"), "wb") as f:
                f.write(b"\0" * (5 * 1024**2))
            size = cm.cache_size(d)
        self.assertIsNotNone(size, "a readable directory must be measurable")
        self.assertGreaterEqual(size, 4 * 1024**2)
        self.assertLess(size, 64 * 1024**2)

    def test_a_partial_du_is_used_rather_than_discarded(self):
        """du exits NON-ZERO on an unreadable entry while still printing a good
        total, and GOCACHE produces that routinely -- gopls and any concurrent
        `go` command write and trim it while the editor is open. Review measured
        a real 450k-file cache exiting 1 over nine unreadable entries.

        Checking the exit status would return None here, and the advisory would
        then be silent on a healthy machine. A partial total can only
        UNDERSTATE, so using it risks a warning that is too quiet while
        discarding it risks no warning at all.
        """
        with tempfile.TemporaryDirectory() as d:
            with open(os.path.join(d, "payload"), "wb") as f:
                f.write(b"\0" * (5 * 1024**2))
            blocked = os.path.join(d, "blocked")
            os.mkdir(blocked)
            os.chmod(blocked, 0o000)
            try:
                if os.access(blocked, os.R_OK):
                    self.skipTest("running with rights that make an unreadable dir readable")
                size = cm.cache_size(d)
            finally:
                os.chmod(blocked, 0o700)
        self.assertIsNotNone(size, "a partial total must be used, not thrown away")
        self.assertGreaterEqual(size, 4 * 1024**2)

    def test_cache_size_reports_none_for_a_directory_that_is_not_there(self):
        self.assertIsNone(cm.cache_size(os.path.join(tempfile.gettempdir(), "no-such-cache-dir")))
        self.assertIsNone(cm.cache_size(""))

    def test_the_warning_mark_sits_a_few_runs_above_the_floor(self):
        """DERIVED, not a second independent guess at the same quantity. The
        first version of this asserted a bare constant against two hand-picked
        bounds and accepted anything in [9, 40) GiB -- it excluded 6-8 GiB and
        nothing else. Tying it to the floor and the measured run cost means a
        drift in either is caught here."""
        self.assertEqual(cm.CACHE_WARN_FREE_BYTES, cm.MIN_FREE_BYTES + 3 * cm.RUN_COST_BYTES)
        self.assertGreater(cm.CACHE_WARN_FREE_BYTES, cm.MIN_FREE_BYTES)
        self.assertGreaterEqual(cm.RUN_COST_BYTES, 8 * 1024**3,
                                "one full run consumes a measured 7.6 GiB")

    # --- a run that could not apply or restore a mutation is NOT a measurement ---
    #
    # gremlins copies the tree per mutant into $TMPDIR. When the disk filled it
    # emitted, per mutant, an ERROR line — and then printed a normal summary and
    # EXITED 0. The gate never looked, so it produced confident, precise, wrong
    # numbers from a run that had measured nothing of the kind.
    #
    # Two distinct corruptions hide in there, and both are silent:
    #   failed to APPLY   — the mutation was never written, so the test ran
    #                       against CLEAN SOURCE. The verdict is about
    #                       unmutated code.
    #   failed to RESTORE — the mutation stays in the workdir file, so every
    #                       SUBSEQUENT mutant in that run is measured against
    #                       already-corrupted source.
    #
    # Measured on internal/rules, identical source both times: the corrupted run
    # tallied 467 of 555 mutant lines and invented seven "stale adjudications" —
    # entries a maintainer would have been told to DELETE, discarding real
    # reasoning and leaving the door open for a genuine future survivor.

    def _with_error(self, kind, *survivors, count=2, **kw):
        """gremlins' REAL error shape, not a tidy invention.

        log.Errorf goes through Writef with NO TRAILING NEWLINE, and
        executor.go formats the cause as "...%s - %s\n\t%v" — a TAB. So
        consecutive errors GLUE onto the previous one's cause line, and a run
        with 88 failures is not 88 tidy lines.

        The first version of this helper wrote one newline-terminated error
        with a "..." continuation, which gremlins cannot produce — and the
        regex written against it matched exactly one error of any number and
        dropped the cause. A fixture that models impossible output tests
        nothing but itself.
        """
        out = gremlins_output(*survivors, **kw)
        head, sep, tail = out.partition("\nMutation testing completed")
        errs = "".join(
            f"ERROR: failed to {kind} mutation at load.go:{i}:14 - RUNNABLE"
            f"\n\tcopy file: no space left on device"
            for i in range(count))
        return head + "\n" + errs + sep + tail

    def test_a_run_that_could_not_apply_a_mutation_is_refused(self):
        eq = equivalents("")
        code, _, err = self.gate(eq, {"./p/": self._with_error("apply", count=2)})
        self.assertEqual(code, 1)
        # The CAUSE, which is the actionable half. Without it, "could not apply
        # a mutation" sends an operator looking for a bug in the gate.
        self.assertIn("no space left on device", err)
        # And ALL of them, not just the first. They glue together in real
        # output, and a regex anchored on the line start counts one.
        self.assertIn("2 mutation(s)", err)
        # And it must NOT be reported as a clean run. The whole defect is that
        # a corrupt measurement looked exactly like a good one.
        self.assertNotIn("zero unadjudicated survivors", err)

    def test_a_run_that_could_not_restore_a_mutation_is_refused(self):
        eq = equivalents("")
        code, out, err = self.gate(eq, {"./p/": self._with_error("restore")})
        self.assertEqual(code, 1)
        self.assertNotIn("zero unadjudicated survivors", out)

    def test_a_corrupt_run_is_refused_even_when_it_reports_survivors_that_are_adjudicated(self):
        """The corruption outranks the verdict.

        This is the shape that actually shipped: a run with errors in it still
        produced a survivor list, the list still matched the equivalents file,
        and the gate said zero unadjudicated survivors — over numbers that
        described partly-unmutated source.
        """
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": self._with_error("apply", ("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, out, err = self.gate(eq, m)
        self.assertEqual(code, 1)
        self.assertNotIn("zero unadjudicated survivors", out)

    def test_an_ordinary_run_is_not_mistaken_for_a_corrupt_one(self):
        """The word ERROR in a test's own output must not red the gate.

        A guard that refuses any line containing "ERROR" would fail on a
        package whose tests legitimately log one — and the fix for THAT is
        usually to weaken the guard, which is how a gate starts lying.
        """
        out = gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))
        noisy = out.replace("      KILLED", "      ERROR: some test logged this\n      KILLED", 1)
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        code, out2, err = self.gate(eq, {"./p/": noisy})
        self.assertEqual(code, 0)
        self.assertIn("zero unadjudicated survivors", out2)

    # --- the tally must account for every mutant line gremlins printed ---
    #
    # A generic check on top of the specific one: mutant count is a pure
    # function of the parsed source, so a summary that does not add up to the
    # per-mutant lines is proof of loss whatever the cause. The ENOSPC run
    # tallied 467 against 555 lines and nothing noticed.

    def test_a_summary_that_does_not_account_for_every_mutant_line_is_refused(self):
        eq = equivalents("")
        # Three mutant lines printed, a summary claiming one.
        out = ("      KILLED CONDITIONALS_NEGATION at x.go:1:1\n"
               "      KILLED CONDITIONALS_NEGATION at x.go:2:1\n"
               "      KILLED CONDITIONALS_NEGATION at x.go:3:1\n"
               "Mutation testing completed in 1 seconds\n"
               "Killed: 1, Lived: 0, Not covered: 0\n"
               "Timed out: 0, Not viable: 0, Skipped: 0\n"
               "Test efficacy: 100.00%\n")
        code, _, err = self.gate(eq, {"./p/": out})
        self.assertEqual(code, 1)
        # Which number is which. Bare "3"/"1" could not tell "printed 3,
        # accounts for 1" from its reverse.
        self.assertIn("printed 3", err)
        self.assertIn("accounts for 1", err)

    def test_a_consistent_summary_passes(self):
        """The control. A tally check that refuses everything gates nothing."""
        eq = equivalents("")
        out = ("      KILLED CONDITIONALS_NEGATION at x.go:1:1\n"
               "  NOT COVERED CONDITIONALS_NEGATION at x.go:2:1\n"
               "   NOT VIABLE CONDITIONALS_NEGATION at x.go:3:1\n"
               "Mutation testing completed in 1 seconds\n"
               "Killed: 1, Lived: 0, Not covered: 1\n"
               "Timed out: 0, Not viable: 1, Skipped: 0\n"
               "Test efficacy: 100.00%\n")
        code, out2, err = self.gate(eq, {"./p/": out})
        self.assertEqual(code, 0, err)

    def test_a_summary_that_does_not_parse_is_fatal_rather_than_skipped(self):
        """Fail closed on format drift, like the counts check beside it."""
        eq = equivalents("")
        out = ("      KILLED CONDITIONALS_NEGATION at x.go:1:1\n"
               "Mutation testing completed in 1 seconds\n"
               "Killed: 1, Lived: 0, Not covered: 0, Ignored: 0\n"
               "Timed out: 0, Not viable: 0, Skipped: 0\n")
        code, _, err = self.gate(eq, {"./p/": out})
        self.assertEqual(code, 1)
        self.assertIn("check the pin", err)

    def test_a_run_that_measured_nothing_is_not_a_clean_sweep(self):
        """Zero mutants is a broken run, not a perfect one.

        Same class as the unresolvable-package and dropped-symlink guards: a
        green line over a run that established nothing. Every gated package has
        hundreds of mutants.
        """
        eq = equivalents("")
        out = ("Mutation testing completed in 1 seconds\n"
               "Killed: 0, Lived: 0, Not covered: 0\n"
               "Timed out: 0, Not viable: 0, Skipped: 0\n")
        code, _, err = self.gate(eq, {"./p/": out})
        self.assertEqual(code, 1)
        self.assertIn("measured nothing", err)

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


class KeyPositionTest(unittest.TestCase):
    """A key must still point at a mutant of the kind it names.

    THE CLASS OF DRIFT THIS CATCHES IS THE DANGEROUS ONE. A key that has merely
    MOVED fails the gate anyway, loudly, as a stale entry beside an
    unadjudicated survivor. A key that has landed on a COMMENT or on a
    different token fails NOTHING: no mutant lives there, so it never appears
    in a survivor list, and the entry sits in the file silently pre-approving
    whatever does land on that line later.

    Measured on the TS side: one wire.ts key pair landed on comment lines in
    two separate episodes three days apart — one of the two on 2026-08-21, both
    of them on 2026-08-24 — and a canvas.ts key landed on one WITHIN AN HOUR of
    being written, because a comment edit above it shifted the line.

    The trap this class is written against is recorded in the equivalents
    file's own header: the hand-rolled version of this check tested
    `startswith("//")`, which a `/**` doc comment does not match, and had no
    branch for the mutator on the entry that had drifted — so it reported "0
    suspect" TWICE over a genuinely broken key. Every rule below therefore has
    a DELIBERATELY BROKEN key that must fail, not only a good key that passes.
    """

    def faults(self, keys):
        """suspect_positions over a hermetic tree, as {location: complaint}."""
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            entries = {k: "reason" for k in keys}
            return {k[1]: fault for k, fault in cm.suspect_positions(entries, root=root)}

    def test_a_key_on_a_line_comment_is_rejected(self):
        faults = self.faults([("p", "a.go:70:1", "CONDITIONALS_BOUNDARY")])
        self.assertIn("a.go:70:1", faults)
        self.assertIn("COMMENT", faults["a.go:70:1"])

    def test_a_key_on_a_block_comment_is_rejected(self):
        """`/*`, which the shipped check's `startswith("//")` did not match."""
        faults = self.faults([("p", "a.go:71:1", "CONDITIONALS_BOUNDARY")])
        self.assertIn("COMMENT", faults.get("a.go:71:1", ""))

    def test_a_key_on_a_doc_comment_continuation_is_rejected(self):
        """A line whose first non-space is `*` — the middle of a /** block, and
        the exact shape the drifted key landed on."""
        faults = self.faults([("p", "a.go:72:3", "CONDITIONALS_BOUNDARY")])
        self.assertIn("COMMENT", faults.get("a.go:72:3", ""))

    def test_a_key_whose_column_holds_a_token_the_mutator_cannot_apply_to(self):
        """80:7 is the `+` of `n := a+b`. ARITHMETIC_BASE lives there;
        CONDITIONALS_BOUNDARY cannot."""
        faults = self.faults([("p", "a.go:80:7", "CONDITIONALS_BOUNDARY")])
        self.assertIn("a.go:80:7", faults)
        self.assertIn("+", faults["a.go:80:7"])

    def test_the_same_column_is_accepted_for_the_mutator_that_does_live_there(self):
        """The control that stops the rule above from being 'reject everything'."""
        self.assertEqual(self.faults([("p", "a.go:80:7", "ARITHMETIC_BASE")]), {})

    def test_a_key_one_column_off_is_rejected(self):
        """The near miss, which is what a hand-re-keyed column actually looks
        like. 10:5 is the `<`; 10:6 is the `b` beside it."""
        self.assertIn("a.go:10:6", self.faults([("p", "a.go:10:6", "CONDITIONALS_BOUNDARY")]))

    def test_a_sound_key_is_accepted(self):
        self.assertEqual(self.faults([("p", "a.go:10:5", "CONDITIONALS_BOUNDARY")]), {})

    def test_the_column_is_a_byte_offset_so_a_tab_counts_as_one(self):
        """Go source is TAB-indented. Counting a tab as a tab stop puts every
        key in every indented file at the wrong column, and the check would
        then reject the whole file — a gate that fails on good data gets
        loosened, and a loosened gate is how this started.

        90 is `\\tif tab<x {`: the `<` is at byte column 8, not 15.
        """
        self.assertEqual(self.faults([("p", "a.go:90:8", "CONDITIONALS_BOUNDARY")]), {})
        self.assertIn("a.go:90:15", self.faults([("p", "a.go:90:15", "CONDITIONALS_BOUNDARY")]))

    def test_a_key_naming_a_line_that_is_not_there_is_rejected(self):
        self.assertIn("a.go:9999:5", self.faults([("p", "a.go:9999:5", "CONDITIONALS_BOUNDARY")]))

    def test_a_key_naming_a_file_that_is_not_there_is_rejected(self):
        self.assertIn("gone.go:10:5", self.faults([("p", "gone.go:10:5", "CONDITIONALS_BOUNDARY")]))

    def test_a_mutator_with_no_rule_is_fatal_rather_than_silently_skipped(self):
        """THE SECOND HALF OF THE RECORDED BUG. The shipped check had no branch
        for one entry's mutator and skipped it without a word, which is how it
        reported "0 suspect" over a key that had drifted.

        A mutator this gate cannot place must therefore FAIL rather than pass.
        Weakening this to "unknown means fine" re-creates the defect exactly.
        """
        faults = self.faults([("p", "a.go:10:5", "NO_SUCH_MUTATOR")])
        self.assertIn("a.go:10:5", faults)
        self.assertIn("NO_SUCH_MUTATOR", faults["a.go:10:5"])

    def test_the_gate_itself_fails_and_names_the_key(self):
        """Through run(), not just the helper: a check nothing calls is a check
        that does not run."""
        eq = equivalents("p  a.go:70:1  CONDITIONALS_BOUNDARY\n    reason\n")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 1, "a key on a comment must FAIL the gate")
        self.assertIn("a.go:70:1", err.getvalue())
        self.assertIn("COMMENT", err.getvalue())
        self.assertNotIn("zero unadjudicated survivors", out.getvalue())

    def test_a_suspect_key_is_not_also_told_to_delete_itself(self):
        """TWO MESSAGES THAT CONTRADICT EACH OTHER ARE WORSE THAN ONE.

        A key on a comment has no mutant, so it is stale as well as suspect,
        and the generic stale line fires beside the position one: "re-key it,
        do not delete it" immediately under "remove the entry". A reader
        following the second loses the reasoning, which is the exact harm this
        whole change exists to stop. The position diagnosis is strictly the
        more informative of the two, so it is the one that survives.

        It costs the gate nothing: the entry is still reported and the gate
        still fails.
        """
        eq = equivalents("p  a.go:70:1  CONDITIONALS_BOUNDARY\n    reason\n")
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            code = cm.run(eq, ["./p/"], runner_for({}), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 1, "a suspect key must still FAIL the gate")
        self.assertIn("a.go:70:1", err.getvalue())
        self.assertNotIn("remove the entry", err.getvalue())

    def test_a_sound_file_of_keys_does_not_red_the_gate(self):
        """The control at the gate level. A position check that refuses good
        keys would be worked around rather than obeyed."""
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reason\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            code = cm.run(eq, ["./p/"], runner_for(m), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        self.assertEqual(code, 0, err.getvalue())

    def test_every_real_adjudication_still_points_at_its_mutant(self):
        """THE ACCEPTANCE ASSERTION, against the real tree and the real file.

        This is the one that would have caught all four of the comment-line
        keys, and it is also the one that keeps the table above honest: a rule
        that rejected any of the 39 recorded adjudications would be a bug in
        the rule, not a finding.
        """
        repo = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        entries = cm.read_equivalents(os.path.join(repo, "tools", "mutation-equivalents.txt"))
        self.assertGreater(len(entries), 30, "the real file must not have gone empty under this")
        self.assertEqual(cm.suspect_positions(entries, root=repo), [],
                         "every key in tools/mutation-equivalents.txt must still point at a "
                         "token its mutator can apply to")


class MovedAdjudicationTest(unittest.TestCase):
    """Pairing a stale entry to the survivor it became, and never advising a
    deletion when a pairing exists.

    THE HARM IS THE DELETE-THE-ADJUDICATION REFLEX. "No longer survives, remove
    the entry" reads as an instruction; obeying it when the mutant merely moved
    throws away reasoning somebody did and leaves a real survivor unexplained.
    Measured: four times in one day, across both gates.

    THE GO KEY CARRIES NO REPLACEMENT TEXT, which is why this gate had only a
    prose hint for so long — `(package, file, mutator)` cannot tell a moved
    mutant from a different one a line away. The signature it pairs on now is
    derived from the SOURCE instead: the operator at the column and the trimmed
    text of the line. Two keys pair when the tree says they are the same
    statement, and the hint is what remains for everything else.
    """

    def gate(self, eq_path, mapping, packages=("./p/",)):
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            source_tree(root)
            code = cm.run(eq_path, list(packages), runner_for(mapping),
                          out=out, err=err, root=root, free_bytes=500 * 1024**3,
                          cache_bytes=0)
        return code, out.getvalue(), err.getvalue()

    def test_a_move_the_source_can_prove_is_reported_as_a_move(self):
        """30 and 40 are the same statement, so a mutant that moved between
        them is pairable by content even with no replacement text in the key."""
        eq = equivalents("p  a.go:30:7  CONDITIONALS_BOUNDARY\n    reasoned\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:40:7"))}
        code, _, err = self.gate(eq, m)
        self.assertEqual(code, 1, "a re-key is still a failure, not a pass")
        self.assertIn("ADJUDICATION MOVED", err)
        self.assertIn("30:7", err)   # where it was
        self.assertIn("40:7", err)   # where it is now
        # And NOT as the two separate failures it used to be, because that
        # framing is what makes deleting a sound adjudication look like the fix.
        self.assertNotIn("no longer survives", err)
        self.assertNotIn("SURVIVED", err)

    def test_a_survivor_on_a_different_statement_is_not_paired(self):
        """THE CAUTION THIS EXTENDS RATHER THAN REPLACES. 10 and 14 carry the
        same mutator and the same operator one screen apart, and they are
        DIFFERENT CLAIMS — pairing them would re-key an adjudication onto a
        mutant nobody judged, which is the failure the gate exists to prevent,
        arrived at by a convenience."""
        eq = equivalents("p  a.go:10:5  CONDITIONALS_BOUNDARY\n    reasoned\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:14:5"))}
        code, _, err = self.gate(eq, m)
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", err)
        self.assertIn("no longer survives", err)
        self.assertIn("not adjudicated", err)

    def test_two_entries_and_two_survivors_pair_by_position_order(self):
        """The AMBIGUOUS class, and the one the gate used to answer with
        "delete them". Four copies of one statement: nothing in the source can
        say which entry became which survivor, and it does not matter, because
        the statements are identical and so the adjudications are
        interchangeable. Deleting two sound adjudications and leaving two real
        survivors unexplained is worse than either ordering."""
        eq = equivalents("p  a.go:30:7  CONDITIONALS_BOUNDARY\n    first\n"
                         "p  a.go:40:7  CONDITIONALS_BOUNDARY\n    second\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:50:7"),
                                     ("CONDITIONALS_BOUNDARY", "a.go:60:7"))}
        code, _, err = self.gate(eq, m)
        self.assertEqual(code, 1)
        # Paired in position order, and SAID so, so a reader can object.
        self.assertIn("POSITION ORDER", err)
        self.assertIn("30:7 -> 50:7", err)
        self.assertIn("40:7 -> 60:7", err)
        self.assertNotIn("no longer survives", err)

    def test_mismatched_counts_still_refuse_to_guess(self):
        """Two entries, one survivor. There is no pairing, only a choice, and
        the gate does not make choices — it reports both sides and fails."""
        eq = equivalents("p  a.go:30:7  CONDITIONALS_BOUNDARY\n    first\n"
                         "p  a.go:40:7  CONDITIONALS_BOUNDARY\n    second\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:50:7"))}
        code, _, err = self.gate(eq, m)
        self.assertEqual(code, 1)
        self.assertNotIn("ADJUDICATION MOVED", err)
        self.assertIn("no longer survives", err)
        self.assertIn("not adjudicated", err)

    def test_a_move_is_never_advice_to_delete(self):
        """The property the whole of Part B is for, asserted on the message
        rather than inferred from the pairing."""
        eq = equivalents("p  a.go:30:7  CONDITIONALS_BOUNDARY\n    reasoned\n")
        m = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:40:7"))}
        _, _, err = self.gate(eq, m)
        self.assertIn("RE-KEY", err)
        self.assertNotIn("remove the entry", err)


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

    def test_the_printed_statuses_are_pinned_on_the_command_line(self):
        """An environment variable must not be able to silence this gate.

        `--output-statuses` defaults to EMPTY and gremlins binds it through
        viper, so GREMLINS_UNLEASH_OUTPUT_STATUSES — or a .gremlins.yaml
        anywhere up the tree — beats an unset flag. MEASURED 2026-08-10 with
        `=k` exported on internal/adventure: the LIVED line vanished while the
        summary still said `Lived: 1`. This gate reads survivors from the
        LINES, so it reported zero survivors and PASSED over a real one.

        Passing the flag explicitly makes the environment irrelevant.
        """
        args = cm.gremlins_args("./internal/rules/", ["./internal/rules/"])
        self.assertIn("--output-statuses", args)
        letters = args[args.index("--output-statuses") + 1]
        # lived, not-covered, timed-out, killed, not-viable, skipped
        # (report/logger.go:58-74). Every status that reaches the summary must
        # also reach the lines, or the tally check below is comparing two
        # different populations.
        self.assertEqual(sorted(letters), sorted("lctkvs"))



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
