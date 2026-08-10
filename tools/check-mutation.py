#!/usr/bin/env python3
"""Fail on any surviving mutant that has not been adjudicated equivalent.

Why a survivor list rather than gremlins' own --threshold-efficacy
-----------------------------------------------------------------
gremlins gates itself on an efficacy PERCENTAGE, which is a ratio: enough
newly-killed mutants can mask a new survivor. internal/engine at 49 killed /
2 lived is 96.08%; at 100 killed / 3 lived it is 97.1% — a real new gap,
passing a 96.0 floor. The 2026-07-24 mutation audit's own standard was "zero
unadjudicated survivors", and that is what this enforces: every survivor must
be named in tools/mutation-equivalents.txt with a reason, or the gate fails.

Patrik chose this over the toolchain threshold (2026-07-28). The precision is
worth the hand-rolled parsing: a masked survivor is exactly the failure this
gate exists to prevent.

What this deliberately does NOT do
----------------------------------
It does not implement mutation testing, schedule it, or interpret Go source.
gremlins does all of that. This reads its output and compares one set against
another — the part gremlins has no opinion about. That boundary is deliberate:
across five review rounds of the coverage gate, 8 of 9 defects were in
hand-rolled enforcement code, and the fix was always to delete it in favour of
the toolchain. Keep this file small enough that it cannot hide a bug.
"""

import os
import re
import subprocess
import sys

# Every package whose mutation run is feasible. This list was ONCE the five
# packages that happened to have been measured while writing the gate — which
# silently dropped internal/campaign, a package the older audit:mutation task
# DID cover and which owns the fold driver, undo, and the poison contract.
# Gating it immediately found three real survivors in c.head's batch
# arithmetic. A gate scoped to what its author touched is not a gate.
#
# internal/harness is STILL excluded, and the reason has changed twice. Keep
# the history, because each version was true when written and each was
# superseded by doing the work rather than by argument.
#
#  1. "Fixed sleeps cost ~70s PER MUTANT, so a run is hours." VOID since
#     2026-07-29: the tests moved into testing/synctest bubbles, the suite
#     went 51s -> 0.7s, and a full run is under four minutes.
#  2. "29 unadjudicated survivors." Worked down to 2 across five passes.
#  3. The actual blocker now: the last real survivor is soak.go:271's
#     CONDITIONALS_NEGATION, which removes a guard that exists to close a
#     RACE — the "has every participant drained the last accepted event"
#     wait before a denial snapshot. A test CAN detect its removal (see
#     TestSoakSlowBroadcastIsNotMistakenForALeak, which delays the fan-out to
#     force the ordering) but only ~60% of the time, measured over repeated
#     runs. The test is stable on correct code, 8/8; it is the DETECTION that
#     is probabilistic.
#
# That is the honest shape of it: synctest made mutation testing feasible
# here, and the same fake clock makes race-guard mutants nondeterministic to
# kill, because "all goroutines durably blocked" erases the very interleaving
# the guard defends against. Gating on a ~60% kill would hand this repo a
# flaky gate, which is the failure mode every other gate here was built to
# avoid. soak.go:611 compounds it: it flips between LIVED and TIMED OUT
# across runs, so even an adjudication for it goes stale at random.
#
# Adding harness needs the race guard's effect made deterministic — not more
# survivors killed.
#
# What is NOT here, with measured survivor counts, is published in
# tools/mutation-scope.md — so the narrowness is a number rather than an
# impression. Three packages turned out to be outside this list on 2026-08-04
# with no recorded reason at all. All three -- internal/rules/conformance,
# internal/adventure and internal/rules -- have since been worked to zero
# unadjudicated survivors and gated. NONE remains outside on no argument.
# cmd/vtt is excluded on the record by ADR-010:96-97, and carries a SECOND,
# independent blocker found 2026-08-05: its scenario tests boot against a
# directory whose only entry is a symlink, and gremlins' workdir copy drops
# symlinks silently — see dropped_symlinks() below. Resolving the package name
# does NOT make it gateable; both routes produce the same constant KILLED
# verdict, and its published "no genuine survivors among the 77 evaluated" was
# retracted for exactly that reason.
#
# tools/toolgen was REMOVED from this list on 2026-08-04. It is `package main`
# in a directory not named `main`, which gremlins cannot resolve — see
# unresolvable_packages() below — so every one of its mutants had been scored
# a false kill for as long as it had been gated. Measured by hand in a renamed
# worktree it is genuinely 9 killed / 0 lived, but the gate was not producing
# that answer and a green line that measures nothing is worse than a gap.
PACKAGES = [
    # Ascending by runtime: the gate fails as fast as it can.
    "./internal/identity/",                # ~16s
    "./internal/adventure/conformance/",   # ~19s
    "./internal/store/",                   # ~33s
    "./internal/engine/",                  # ~38s
    "./internal/rules/conformance/",       # ~48s
    "./internal/gateway/",                 # ~57s
    "./internal/adventure/",               # ~60s
    "./internal/campaign/",                # ~91s
    "./internal/rules/",                   # ~393s -- the interpreter, 553 mutants
    "./internal/mcp/",                     # ~857s
]

# `LIVED CONDITIONALS_BOUNDARY at apply.go:147:15`
LIVED_RE = re.compile(r"^\s*LIVED\s+(\S+)\s+at\s+(\S+)\s*$")

# `   TIMED OUT CONDITIONALS_NEGATION at server.go:158:9`
TIMED_OUT_RE = re.compile(r"^\s*TIMED OUT\s+(\S+)\s+at\s+(\S+)\s*$")

# `Timed out: 58, Not viable: 0, Skipped: 0`
TIMEOUT_RE = re.compile(r"^Timed out:\s*(\d+)", re.M)
KILLED_RE = re.compile(r"^Killed:\s*(\d+),\s*Lived:\s*(\d+)", re.M)

# A timed-out mutant is one gremlins abandoned because the suite ran too long
# under it. Patrik's ruling (2026-07-28): count those as detections rather than
# failing the gate on them, since in this codebase they are overwhelmingly
# mutants that hang a socket or a wait.
#
# BUT THEY ARE NOT RELIABLY DETECTIONS, and an earlier version of this comment
# claimed they were ("the suite did not pass"). A review disproved it with a
# working counterexample: a lenient assertion over a value whose upper bound
# the mutation removes —
#
#     if d > ceiling { d = ceiling }   // mutate away the clamp
#     time.Sleep(d)                    // now sleeps 60s
#     ... test asserts only `got >= 0` ...
#
# gremlins reports TIMED OUT; applying that same mutation by hand, the suite
# PASSES in 60s. The mutant genuinely survives and this gate would score it as
# detected. The shape is not exotic — deadlines, backoffs and handshake
# timeouts are exactly what the gated packages are full of.
#
# So every timed-out mutant is PRINTED with its location. They are not failures
# per the ruling, but they are the set this gate did not actually measure, and
# discarding them is how a survivor disappears silently. gremlins names each
# one; there is no excuse for the gate to know less than its own input.
#
# The cap is the second guard. Counting timeouts as kills inverts into the
# failure observed here first: two goroutine-waiting tests took internal/mcp
# from 57 killed / 8 lived / 0 timed out to 6 / 0 / 58, with gremlins still
# printing "Test efficacy: 100.00%" (it computes killed/(killed+lived) and
# ignores timeouts). Scoring that as perfect would be worse than useless.
# Measured proportions, two orders apart:
#     mcp (broken)   58 of 64 timed out = 91%
#     gateway         6 of 33 timed out = 18%
#     mcp (fixed)     0 of 64 timed out =  0%
# A MAJORITY fails as an unusable measurement. The denominator is
# killed+lived+timed_out; "not covered" mutants are excluded by definition,
# since no test reaches them. Exactly 50% is treated as acceptable — the
# boundary is pinned by test, not left to the reader.
MAX_TIMEOUT_FRACTION = 0.50


class EquivalentsError(Exception):
    pass


def read_equivalents(path):
    """Return {(package, location, mutator): reason}.

    Entries are a header line plus an indented reason. The reason is REQUIRED:
    an equivalence claim without a stated observable is exactly the shape that
    produced two wrong adjudications on 2026-07-27.
    """
    entries = {}
    pending = None
    with open(path) as fh:
        for lineno, raw in enumerate(fh, 1):
            if not raw.strip() or raw.lstrip().startswith("#"):
                continue
            if raw[0].isspace():
                if pending is None:
                    raise EquivalentsError(
                        f"{path}:{lineno}: reason line with no entry above it")
                entries[pending] = raw.strip()
                pending = None
                continue
            if pending is not None:
                raise EquivalentsError(
                    f"{path}:{lineno}: entry {pending[1]} has no reason line — state the "
                    f"observable that makes it equivalent, or write the test instead")
            parts = raw.split()
            if len(parts) != 3:
                raise EquivalentsError(
                    f"{path}:{lineno}: want '<package>  <file:line:col>  <MUTATOR>', got {raw.strip()!r}")
            pending = (parts[0], parts[1], parts[2])
    if pending is not None:
        raise EquivalentsError(f"{path}: entry {pending[1]} at end of file has no reason line")
    return entries


def _norm_pkg(name):
    """Normalise a package as written to the form survivors carry.

    read_equivalents takes the package VERBATIM while the survivor loop stores
    `pkg.strip("./").rstrip("/")`, so `./p/` and `p` are different keys. That is
    a real footgun and the first attempt at pairing here silently matched
    nothing because of it — so anything comparing the two sides normalises
    both, and the hint below diagnoses the mismatch by name rather than
    leaving somebody to find it.
    """
    return name.strip("./").rstrip("/")


def stale_entry_hint(name, location, mutator, unadjudicated):
    """A HINT that a stale adjudication may be the same mutant, moved. Never a claim.

    THE HARM THIS ADDRESSES is the delete-the-adjudication reflex. "No longer
    survives, remove the entry" reads as an instruction, and when an edit has
    merely SHIFTED a mutant, obeying it throws away reasoning somebody did and
    leaves the survivor unexplained. That happened four times in one day.

    THE TS GATE CAN DO BETTER and this one deliberately does not. Over there the
    key carries the replacement text, so a move can be identified and NAMED.
    Here the key is (package, file:line:col, mutator) with no replacement, so
    "same file, same mutator" cannot tell a moved mutant from a different one a
    line away — and check_mutation_test.py's
    test_adjudication_is_matched_on_all_three_fields exists precisely to pin
    that a survivor one line from an adjudication is a DIFFERENT claim. An
    earlier attempt paired them anyway and broke that test; editing the test to
    fit would have been weakening a gate's own test to pass a change.

    So this returns TEXT and nothing else. Both the stale entry and the
    unadjudicated survivor are still reported, and the gate still fails. The
    reader gets a lead; the gate keeps its verdict.
    """
    pkg = _norm_pkg(name)
    file_ = location.rsplit(":", 2)[0]

    spelling, elsewhere = False, []
    for other_name, other_loc, other_mutator in unadjudicated:
        if _norm_pkg(other_name) != pkg or other_mutator != mutator:
            continue
        if other_loc == location:
            # Same normalised package, same location, same mutator — and the
            # entry is stale while this one is unadjudicated. The two package
            # strings must therefore differ VERBATIM. Checked rather than
            # assumed, so a future caller passing an identical spelling cannot
            # be told its spelling is wrong.
            spelling = spelling or other_name != name
        elif other_loc.rsplit(":", 2)[0] == file_:
            elsewhere.append(other_loc)

    if spelling:
        # NOT a move, and not a guess either. If the two spellings agreed the
        # keys would be equal, the survivor would be claimed and this entry
        # would not be stale — so the mismatch is deduced, not suspected.
        # Hedging it would invite the reader to ignore correct, actionable
        # advice, which is why the "hint, not a match" wording belongs on the
        # other branch and not this one.
        return (f". THE SAME MUTANT SURVIVES UNADJUDICATED AT THAT EXACT LOCATION: this entry "
                f"spells the package {name!r} while survivors carry {pkg!r}, so it matches "
                f"nothing. Write it as {pkg!r} rather than deleting the entry. A DEDUCTION, not "
                f"a guess: were the spellings equal the keys would be equal, the survivor would "
                f"be claimed, and this entry would not be stale.")
    if elsewhere:
        where = ", ".join(sorted(set(elsewhere)))
        return (f". NOTE: {mutator} also survives unadjudicated in that file, at {where} — if an "
                f"edit SHIFTED this mutant, RE-KEY this entry and re-check that its reason still "
                f"holds, rather than deleting it. This is a hint, not a match: unlike the TS gate, "
                f"these keys carry no replacement text, so same-file-same-mutator cannot tell a "
                f"moved mutant from a different one a line away.")
    return ""


def parse_survivors(output):
    """Return [(location, mutator)] from one gremlins run's output."""
    survivors = []
    for line in output.splitlines():
        m = LIVED_RE.match(line)
        if m:
            survivors.append((m.group(2), m.group(1)))
    return survivors


def parse_timed_out(output):
    """Return [(location, mutator)] for mutants gremlins abandoned.

    These are counted as detections per the ruling above, but they are NOT
    reliably so — see MAX_TIMEOUT_FRACTION's comment for the counterexample.
    They are the set this run did not actually measure, so they get printed.
    """
    out = []
    for line in output.splitlines():
        m = TIMED_OUT_RE.match(line)
        if m:
            out.append((m.group(2), m.group(1)))
    return out


def run_package(pkg, runner):
    """Run gremlins over pkg; return (survivors, timed_out).

    Raises if the run did not complete, or if so many mutants timed out that
    the result says nothing about the ones that did not.
    """
    out = runner(pkg)
    # gremlins exits non-zero when mutants survive and no threshold is set, so
    # the exit code is not a failure signal here — the survivor list is.
    if "Mutation testing completed" not in out:
        raise EquivalentsError(
            f"gremlins produced no summary for {pkg} — it did not complete:\n{out}")

    tm = TIMEOUT_RE.search(out)
    km = KILLED_RE.search(out)
    if not tm or not km:
        # Failing open here would re-create the exact blind spot this gate was
        # built after: a grep that never looked at the `Timed out:` line
        # reported two packages as "100% efficacy" while a quarter to a
        # majority of their mutants went unevaluated.
        raise EquivalentsError(
            f"{pkg}: could not parse gremlins' summary counts (Killed/Lived/Timed out) — "
            f"the output format changed, so the timeout check cannot run and this result "
            f"cannot be trusted. gremlins is pinned at v0.6.0 for this reason; check the pin.")
    timed_out = int(tm.group(1))
    evaluated = int(km.group(1)) + int(km.group(2))
    total = evaluated + timed_out
    if total > 0 and timed_out / total > MAX_TIMEOUT_FRACTION:
        raise EquivalentsError(
            f"{pkg}: {timed_out} of {total} mutants TIMED OUT ({timed_out / total:.0%}) — "
                f"a majority, so this is a broken measurement rather than detection, and "
                f"gremlins would still report a clean efficacy (it computes killed/(killed+"
                f"lived) and ignores timeouts). The usual cause is a test that BLOCKS on a "
                f"long deadline: a broken mutant hangs it instead of failing it. Make such "
            f"tests fail fast — dropping two mcp deadlines from 10s to 3s took that "
            f"package from 58 timeouts to 1.")
    return parse_survivors(out), parse_timed_out(out)


# gremlins derives each mutant's timeout from a BASELINE run of the package
# suite, and that baseline moves with machine state: in a full gate run, where
# mcp follows seven warm packages, the baseline is fast and the timeout tight;
# run alone after a code change, compilation inflates it and the timeout is
# generous. internal/mcp measured 58 timeouts, then 1, then 58 again across
# three runs of identical code — the measurement, not the code, was moving.
#
# 30 rather than 10 buys headroom so a slow-but-correct mutant is not mistaken
# for a hung one. It costs wall clock and nothing else, which per Patrik's
# ruling (2026-07-28) is not a reason to prefer the flakier number.
TIMEOUT_COEFFICIENT = "30"


PACKAGE_CLAUSE_RE = re.compile(r"^package\s+(\w+)")


def declared_package(pkg_dir):
    """The package name declared by a directory's non-test .go files."""
    try:
        names = sorted(os.listdir(pkg_dir))
    except OSError:
        return None
    for name in names:
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        with open(os.path.join(pkg_dir, name)) as fh:
            for line in fh:
                m = PACKAGE_CLAUSE_RE.match(line)
                if m:
                    return m.group(1)
    return None


def unresolvable_packages(packages, root="."):
    """Gated packages gremlins cannot resolve a test target for.

    gremlins picks a mutant's test target by walking UP from the mutated file
    for a directory whose NAME equals the declared package name. A `package
    main` in a directory not named `main` has no such ancestor, so gremlins
    falls back to the bare MODULE PATH and runs `go test github.com/owner/repo`
    — which does not resolve, exits 1, and exit 1 is precisely what gremlins
    scores as KILLED. Every mutant becomes a false kill in ~11ms, and the gate
    prints a clean measurement for a run that never executed a single test.

    This is the gate holding itself to its own standard. It exists because the
    failure is INVISIBLE from the outside: the output is indistinguishable from
    a genuinely well-tested package, and sampling a killable mutant to check
    cannot detect it — a constant "everything dies" verdict passes that check
    too. Only a provably EQUIVALENT mutant reported as killed gives it away.

    Found 2026-08-04 by review, in a change adding cmd/vtt to PACKAGES on the
    strength of "91 killed, 0 lived, ~9s" — while `go test ./cmd/vtt/` alone
    takes 29s. tools/toolgen had been in the gated set with the same defect,
    never measured. tools/mutation-scope.md carries both, with the manual
    measurements taken in their place.

    Returns [(pkg, declared_name, directory_name), ...]; empty is good.
    """
    bad = []
    for pkg in packages:
        rel = _norm_pkg(pkg)
        declared = declared_package(os.path.join(root, rel))
        if declared is None:
            # No Go source to read. run_package will fail on its own terms;
            # inventing a second diagnosis here would only obscure that one.
            continue
        dirname = os.path.basename(rel)
        if declared != dirname:
            bad.append((pkg, declared, dirname))
    return bad


# Symlinks that exist in the tree and are accounted for. The value is the
# REASON, and a reason must name which packages the symlink makes unmeasurable
# — that is the entire point of recording it. A bare "we know about this one"
# would let the next person gate a package this entry already forbids.
ALLOWED_SYMLINKS = {
    # Empty, and that is the healthy state. The repo carried exactly one
    # tracked symlink — scenarios/testdata/dnd45e-minimal-adventures/
    # goblin-ambush — which existed only because `serve --adventures-dir
    # ./adventures` could not boot a mixed-ruleset library. loadAdventuresDir
    # now selects by ruleset, so the fixture is gone and cmd/vtt's tests no
    # longer depend on a path gremlins drops.
    #
    # An entry here must name which packages the symlink makes unmeasurable;
    # that is the whole point of recording one rather than merely tolerating
    # it. Prefer removing the symlink to adding an entry.
}

# Not part of the Go module, and node_modules is full of symlinks: walking it
# would make this guard both slow and permanently red.
UNWALKED_DIRS = {".git", "node_modules"}


def dropped_symlinks(root=".", allowed=None):
    """Symlinks in the tree that are not recorded in ALLOWED_SYMLINKS.

    gremlins copies the module before mutating it, and that copy DROPS
    symlinks: `copyPath` (workdir.go:142) switches on `mode.IsDir()` and
    `mode.IsRegular()`, a symlink is neither, so it falls through the switch
    and is skipped with no error. The path is simply absent from the copy.

    A test that reads it then fails in the copy for EVERY mutant, exiting 1 —
    and exit 1 is precisely what gremlins scores as KILLED
    (getTestFailedStatus, executor.go:257). So the package reports perfect
    efficacy having detected nothing. Same constant verdict as
    unresolvable_packages() above, reached by a different route, and just as
    invisible from the output.

    Only exit 1 does this. Exit 2 is NOT VIABLE and every OTHER non-zero exit
    is scored LIVED, so the neighbouring failure inverts: a suite killed by a
    signal yields false SURVIVORS rather than false kills. The distinction is
    worth keeping straight — the two mistakes have opposite symptoms.

    Found 2026-08-05: cmd/vtt's published "75 killed, no genuine survivors
    among the 77 evaluated" was this, not a well-tested package. Its scenario
    tests boot against scenarios/testdata/dnd45e-minimal-adventures, whose only
    entry is a symlink, and in the copy that directory is empty.

    Returns repo-relative paths, sorted; empty is good.
    """
    if allowed is None:
        allowed = ALLOWED_SYMLINKS
    for path, reason in sorted(allowed.items()):
        if not reason or not reason.strip():
            raise EquivalentsError(
                f"ALLOWED_SYMLINKS[{path!r}] has no reason. An excuse with no argument is how "
                f"a real gap becomes a permanent one in writing — the reason must name which "
                f"packages this symlink makes unmeasurable.")

    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        # Check BEFORE pruning. A symlink NAMED `node_modules` is a dropped
        # symlink like any other, and skipping it for its name would give the
        # guard a hole shaped exactly like its own exclusion list.
        for name in list(dirnames) + filenames:
            full = os.path.join(dirpath, name)
            if not os.path.islink(full):
                continue
            rel = os.path.relpath(full, root).replace(os.sep, "/")
            if rel not in allowed:
                found.append(rel)
        dirnames[:] = [d for d in dirnames if d not in UNWALKED_DIRS]
    return sorted(found)


def gremlins_args(pkg, packages=PACKAGES):
    """The gremlins invocation for one package, excluding its GATED children.

    `unleash ./internal/rules/` RECURSES into subdirectories and reports those
    mutants relative to the package it was given — `conformance/conformance.go`
    rather than `internal/rules/conformance/conformance.go`. With both the
    parent and the child in PACKAGES that is wrong twice: the same mutant is
    measured under two different keys, so an adjudication written for the child
    does not match the parent's report and an already-excused survivor is
    called unadjudicated; and it costs the runtime twice (internal/adventure's
    run carried 23 of its child's mutants, internal/rules' carried 60).

    Only GATED children are excluded. A subdirectory that is not separately in
    PACKAGES is measured here or nowhere, and dropping it would trade a visible
    gap for an invisible one — the failure this whole gate exists to prevent.

    Exclusion goes through gremlins' own --exclude-files rather than filtering
    its output, per this file's header: 8 of 9 defects across five review
    rounds of the coverage gate lived in hand-rolled enforcement code, and the
    fix was always to hand the job back to the toolchain.

    THE REGEXP IS ANCHORED TO THE PACKAGE-RELATIVE PATH, which is what makes a
    bare `^conformance/` correct and is the one thing here no unit test can
    own -- it is a property of the pinned gremlins, not of this code. In
    v0.6.0: engine.go:73 roots an os.DirFS at the directory unleash was given,
    engine.go:104 walks it from ".", and exclusion/rules.go:43 does a plain
    regexp MatchString against that relative path with no (?m), so `^` is
    start-of-text. Consequently `conformance_helpers.go` cannot match (needs a
    literal `/`) and `foo/conformance/bar.go` cannot either (not at position
    0). RE-VERIFY THOSE THREE LINES ON A VERSION BUMP; the tests below assert
    the argv, not gremlins' honouring of it.
    """
    args = ["go", "tool", "gremlins", "unleash", pkg,
            "--workers", "1", "--timeout-coefficient", TIMEOUT_COEFFICIENT]
    parent = _norm_pkg(pkg)
    for other in packages:
        child = _norm_pkg(other)
        # Trailing slash on the prefix so internal/rulesets is not read as a
        # child of internal/rules.
        if child != parent and child.startswith(parent + "/"):
            rel = child[len(parent) + 1:]
            args += ["--exclude-files", f"^{re.escape(rel)}/"]
    return args


def default_runner(pkg):
    proc = subprocess.run(gremlins_args(pkg), capture_output=True, text=True)
    return proc.stdout + proc.stderr


def run(equivalents_path, packages=PACKAGES, runner=default_runner,
        out=sys.stdout, err=sys.stderr, root="."):
    # BEFORE anything is measured: a package gremlins cannot resolve reports
    # every mutant as killed, so letting the loop below run would print `ok`
    # lines that establish nothing.
    unresolvable = unresolvable_packages(packages, root=root)
    if unresolvable:
        for pkg, declared, dirname in unresolvable:
            print(f"check:mutation: {pkg} declares `package {declared}` in a directory named "
                  f"`{dirname}`. gremlins resolves a test target by matching the directory name "
                  f"to the package name, so it falls back to the bare module path, `go test` "
                  f"exits 1 for an unresolvable package, and EVERY mutant is scored KILLED in "
                  f"milliseconds. The measurement would be worthless. Remove it from PACKAGES "
                  f"and record it in tools/mutation-scope.md.", file=err)
        return 1

    # Also before anything is measured: a symlink gremlins drops silently makes
    # every test that reads it fail in the copy, under every mutant.
    try:
        stray = dropped_symlinks(root=root)
    except EquivalentsError as exc:
        print(f"check:mutation: {exc}", file=err)
        return 1
    if stray:
        for path in stray:
            print(f"check:mutation: {path} is a symlink. gremlins' workdir copy switches on "
                  f"IsDir()/IsRegular() and drops symlinks with no error, so any test that reads "
                  f"this path fails in the copy under EVERY mutant — and exit 1 is what gremlins "
                  f"scores as KILLED. Replace it with a real file or directory. If it must stay "
                  f"AND it is committed, record it in ALLOWED_SYMLINKS with the packages it makes "
                  f"unmeasurable — never record an untracked local one, which would red the gate "
                  f"for every other clone.", file=err)
        return 1

    try:
        equivalents = read_equivalents(equivalents_path)
    except (EquivalentsError, OSError) as exc:
        print(f"check:mutation: {exc}", file=err)
        return 1

    unadjudicated = []
    claimed = set()
    for pkg in packages:
        name = _norm_pkg(pkg)
        try:
            survivors, timed_out = run_package(pkg, runner)
        except EquivalentsError as exc:
            print(f"check:mutation: {exc}", file=err)
            return 1
        for location, mutator in survivors:
            key = (name, location, mutator)
            if key in equivalents:
                claimed.add(key)
            else:
                unadjudicated.append(key)
        adjudicated = len([s for s in survivors if (name, s[0], s[1]) in equivalents])
        note = f", {len(timed_out)} TIMED OUT (counted as killed, NOT measured)" if timed_out else ""
        print(f"ok  {name}: {len(survivors)} survivor(s), {adjudicated} adjudicated{note}", file=out)
        # Name them. A timed-out mutant can be a genuine survivor (see
        # MAX_TIMEOUT_FRACTION), so the one thing the gate must not do is let
        # them vanish between gremlins' output and this report.
        for location, mutator in timed_out:
            print(f"    timed out: {name} {location} {mutator} — not evaluated; if this "
                  f"persists, make the test that blocks under it fail fast", file=out)

    failed = False
    for name, location, mutator in unadjudicated:
        print(f"check:mutation: {name} {location} {mutator} SURVIVED and is not adjudicated — "
              f"write a test that kills it, or (only if no observable can distinguish it) add it "
              f"to {equivalents_path} with the reason", file=err)
        failed = True

    # A stale entry means the mutant is gone — the code changed, or someone
    # finally killed it. Left in place it silently pre-approves a future
    # survivor at the same location.
    for name, location, mutator in sorted(set(equivalents) - claimed):
        print(f"check:mutation: {equivalents_path} lists {name} {location} {mutator}, which no "
              f"longer survives — remove the entry so it cannot pre-approve a future survivor "
              f"at that location{stale_entry_hint(name, location, mutator, unadjudicated)}",
              file=err)
        failed = True

    if failed:
        return 1
    print(f"\ncheck:mutation: {len(packages)} packages, zero unadjudicated survivors.", file=out)
    return 0


def main(argv):
    if len(argv) != 2:
        print("usage: check-mutation.py <equivalents-file>", file=sys.stderr)
        return 2
    return run(argv[1])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
