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
import shutil
import tempfile
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
    # internal/mapdef (maps-as-geometry): the arc's core package, and pure
    # validation logic — exactly what a mutation gate is best at. It was in
    # NEITHER this list nor mutation-scope.md, whose stated job is to publish
    # what sits outside the gate, so it was silently ungated rather than
    # deliberately excluded. Found by the whole-branch review.
    "./internal/mapdef/",
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
# The floor below which a mutation run must not START.
#
# MEASURED 2026-08-10 on this machine, with a freshly emptied cache: one full
# `task check` consumed 7.6 GiB of free space and left GOCACHE 4 GiB larger than
# it found it. gremlins copies the whole tree per mutant into $TMPDIR and every
# mutant is a distinct binary, so the cache grows without bound across runs —
# which is how it reached 82 GB and filled a 228 GB volume.
#
# 16 GiB is that measured cost with room for one more run, not a round number
# picked for looking careful. It is deliberately generous: the cost of refusing
# a run that would have squeaked through is a clear message and a `go clean
# -cache`, while the cost of admitting one that cannot finish is a CORRUPT
# VERDICT that looks exactly like a good one — and, when the volume actually
# fills, a machine on which Bash itself stops working, so the failure does not
# even present as a gate failure.
MIN_FREE_BYTES = 16 * 1024**3

# One full gate run's measured consumption of free space, rounded up from the
# 7.6 GiB recorded above. Named so the warning below is DERIVED from it rather
# than being a second independent guess about the same run.
RUN_COST_BYTES = 8 * 1024**3

# Where the disk gets a WORD, well before MIN_FREE_BYTES gets a veto.
#
# The floor fails closed, but only at a cliff, and by then the volume may be too
# full for Bash itself to work. This is the ramp: three more runs' worth of room
# above the floor, so there is time to act while acting is still easy.
#
# TRIGGERED ON FREE SPACE, NOT ON CACHE SIZE, which was a review finding rather
# than the first design. GOCACHE is MACHINE-GLOBAL — every Go project on the box
# shares it — so "the cache is over N GiB" says nothing about whether THIS
# volume is in trouble. A developer with two other Go repos would sit above any
# useful threshold permanently with 100 GiB free, and be told every single run
# to `go clean -cache`, throwing away every project's cache to solve nothing. A
# warning that fires every time is one people learn to scroll past, which costs
# the warning that mattered. Free space is the quantity that actually ran out;
# cache size is a proxy that tracks it only on a small volume.
#
# ADVISORY ONLY. It never changes the exit code. Making it refuse would be a new
# gate, and a gate change is its own reviewed decision (CLAUDE.md rule 2).
CACHE_WARN_FREE_BYTES = MIN_FREE_BYTES + 3 * RUN_COST_BYTES


def go_cache():
    """GOCACHE, from the environment or the toolchain.

    Returns "" if the toolchain reports none. RAISES if `go` is not on PATH,
    deliberately: swallowing that would make _disk_targets silently drop GOCACHE
    from the floor's targets, narrowing a gate by accident.
    """
    return os.environ.get("GOCACHE") or subprocess.run(
        ["go", "env", "GOCACHE"], capture_output=True, text=True).stdout.strip()


def cache_size(path):
    """Bytes under path, or None if that cannot be measured.

    Only ever called to enrich a warning that has ALREADY been decided, so None
    costs one clause of a message rather than the message.

    du rather than an os.walk: the build cache holds hundreds of thousands of
    small files. Measured 0.09s at 4.31 GiB warm; ~25s cold on a 450k-file tree.

    THE EXIT STATUS IS NOT CONSULTED, only stdout. Review measured a real
    450k-file cache where du exited 1 over nine unreadable entries while
    printing a perfectly good total, and a file vanishing mid-walk does the
    same — not exotic here, since gopls and any concurrent `go` command write
    and trim this directory continuously. A partial total can only UNDERSTATE,
    so parsing one risks a warning that is too quiet, while discarding it risks
    no warning at all.
    """
    if not path or not os.path.isdir(path):
        return None
    try:
        proc = subprocess.run(["du", "-sk", path], capture_output=True,
                              text=True, timeout=120)
    except (OSError, subprocess.SubprocessError):
        return None
    try:
        return int(proc.stdout.split()[0]) * 1024
    except (IndexError, ValueError):
        return None



# A run that could not APPLY or RESTORE a mutation is not a measurement.
#
# gremlins copies the tree per mutant into $TMPDIR. When the disk filled it
# emitted one of these per mutant, then printed a normal summary and EXITED 0 —
# so the gate produced confident, precise numbers from a run that had measured
# something other than what it claimed.
#
# The two halves fail differently and both are silent:
#   failed to APPLY   — the mutation was never written, so the test ran against
#                       CLEAN SOURCE and the verdict describes unmutated code.
#   failed to RESTORE — the mutation stays in the workdir file, so EVERY
#                       SUBSEQUENT mutant in that run is measured against
#                       already-corrupted source.
#
# Anchored on gremlins' own phrasing rather than on the word ERROR, which a
# package's own tests may legitimately log — a guard that reds the gate on
# those gets weakened, and a weakened gate is how this started.
# gremlins' real shape, read from the source rather than guessed:
#
#   log.Errorf -> eWritef("%s: %s", ...)   -- Writef, NO TRAILING NEWLINE
#   executor.go:178 -> "failed to apply mutation at %s - %s\n\t%v"  -- a TAB
#
# So the cause is tab-indented, and because the error itself never ends a line,
# CONSECUTIVE ERRORS GLUE onto the previous one's cause. A run with 88 failures
# is not 88 tidy lines. An earlier version of this anchored on `^` and looked
# for a `...` continuation — a shape gremlins cannot produce — and matched
# exactly ONE of them, dropping "no space left on device" with it. The phrase
# is the anchor; the line start is not.
# The PHRASE ONLY, so the count is a count. Matching the cause line too made a
# single match swallow the next glued error, and 88 failures reported as 1.
MUTATION_IO_ERROR_RE = re.compile(r"failed to (?:apply|restore) mutation\b")

# Every per-mutant verdict line gremlins prints, cross-checked against the
# summary counts.
#
# WHAT THIS DOES AND DOES NOT CATCH, because the first version of this comment
# claimed the wrong thing. It does NOT catch a disk-exhausted run: the printed
# lines and the summary come from the same slice (engine.go:229), and a mutant
# that failed to apply is dropped from both, so the two agree by construction
# however corrupt the run. The errors above are what catch that.
#
# What it DOES catch is a printed set that no longer matches the counted set —
# output filtering (see --output-statuses, now pinned) and format drift. It is
# the check that would have caught `GREMLINS_UNLEASH_OUTPUT_STATUSES=k`
# silencing every survivor: 74 lines against 75 counted.
MUTANT_LINE_RE = re.compile(
    r"^\s*(?:KILLED|LIVED|TIMED OUT|NOT COVERED|NOT VIABLE|SKIPPED)\s+\S+\s+at\s+\S+\s*$",
    re.M)

SUMMARY_COUNTS_RE = re.compile(
    r"^Killed:\s*(\d+),\s*Lived:\s*(\d+),\s*Not covered:\s*(\d+)\s*$"
    r"\s*^Timed out:\s*(\d+),\s*Not viable:\s*(\d+),\s*Skipped:\s*(\d+)\s*$", re.M)

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


def _first_io_error(out):
    """The first apply/restore error WITH its cause, as one readable line.

    Cut at the next `ERROR:` because they glue: gremlins' Errorf writes no
    trailing newline, so a sibling error starts partway through the previous
    one's tab-indented cause.
    """
    m = MUTATION_IO_ERROR_RE.search(out)
    if not m:
        return ""
    tail = out[m.start():m.start() + 400]
    nxt = tail.find("ERROR:", 1)
    if nxt != -1:
        tail = tail[:nxt]
    return " ".join(tail.split())


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

    # BEFORE anything is parsed. A run carrying these errors has no verdict to
    # extract, and reading one out of it is the defect: the numbers look exactly
    # like a good run's.
    broken = MUTATION_IO_ERROR_RE.findall(out)
    if broken:
        raise EquivalentsError(
            f"{pkg}: gremlins could not apply or restore {len(broken)} mutation(s), so this run "
            f"is NOT a measurement and no verdict may be read from it. A failed APPLY means the "
            f"test ran against CLEAN source; a failed RESTORE means every subsequent mutant in "
            f"the run was measured against already-corrupted source. The usual cause is a full "
            f"disk — gremlins copies the tree per mutant into $TMPDIR, and `task check:mutation` "
            f"grows GOCACHE without bound (82 GiB measured). Check free space, `go clean -cache`, "
            f"and run it again. First error:\n    {_first_io_error(out)}")

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
    # The summary must account for every per-mutant line above it. Generic
    # where the check above is specific: it catches silent disappearance
    # whatever the cause, and it costs one regex.
    sm = SUMMARY_COUNTS_RE.search(out)
    if not sm:
        # FAIL CLOSED, like the counts check twelve lines above. Skipping the
        # tally on an unrecognised summary would put the gate back where it
        # started: quietly not looking.
        raise EquivalentsError(
            f"{pkg}: gremlins' summary lines did not parse, so the per-mutant tally cannot be "
            f"checked and this result cannot be trusted. The output format changed; gremlins is "
            f"pinned at v0.6.0 for exactly this reason, so check the pin.")
    claimed = sum(int(g) for g in sm.groups())
    printed = len(MUTANT_LINE_RE.findall(out))
    if printed == 0:
        # Every gated package has hundreds of mutants. Zero is not a clean
        # sweep, it is a run that measured nothing — a mis-scoped
        # --exclude-files, or a package that failed to parse.
        raise EquivalentsError(
            f"{pkg}: gremlins printed no per-mutant verdicts at all. That is a run which "
            f"measured nothing, not a package with nothing to measure — check --exclude-files "
            f"and that the package builds.")
    if claimed != printed:
        raise EquivalentsError(
                f"{pkg}: gremlins printed {printed} per-mutant line(s) but its summary accounts "
                f"for {claimed}. Mutant count is a pure function of the parsed source, so a "
                f"summary that does not add up is proof that mutants went missing — the result "
                f"cannot be trusted whatever the cause. A disk-exhausted run tallied 467 of 555 "
                f"this way and reported a clean efficacy.")

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
            "--workers", "1", "--timeout-coefficient", TIMEOUT_COEFFICIENT,
            # WHICH VERDICTS GET PRINTED, pinned explicitly — l,c,t,k,v,s being
            # lived, not-covered, timed-out, killed, not-viable, skipped
            # (report/logger.go:58-74). 'r' (runnable) is omitted: it only
            # occurs under --dry-run.
            #
            # This closes a hole that had nothing to do with disks. The flag
            # defaults to EMPTY and gremlins binds it through viper, so
            # GREMLINS_UNLEASH_OUTPUT_STATUSES in the environment — or a
            # .gremlins.yaml anywhere up the tree — beats an unset flag. With
            # `=k` exported, MEASURED 2026-08-10 on internal/adventure, the
            # LIVED line vanishes while the summary still says `Lived: 1`. This
            # gate reads survivors from the LINES, so it would have reported
            # zero survivors and PASSED over a real one. An environment
            # variable could silence the gate.
            "--output-statuses", "lctkvs"]
    parent = _norm_pkg(pkg)
    for other in packages:
        child = _norm_pkg(other)
        # Trailing slash on the prefix so internal/rulesets is not read as a
        # child of internal/rules.
        if child != parent and child.startswith(parent + "/"):
            rel = child[len(parent) + 1:]
            args += ["--exclude-files", f"^{re.escape(rel)}/"]
    return args


def clear_test_cache_args():
    """The command that makes every mutant's deadline mean the same thing.

    -testcache, NOT -cache, and the difference is the whole cost of this fix:
    `go clean -testcache` discards cached test RESULTS and leaves every
    compiled artifact alone, so nothing is rebuilt. `go clean -cache` throws
    away the build cache and costs minutes of recompilation on every gate run.
    """
    return ["go", "clean", "-testcache"]


def clear_test_cache():
    """Discard cached test results, so gremlins' baseline is a real measurement.

    WHY THIS EXISTS, read from gremlins v0.6.0 and then measured. Every mutant's
    deadline is `cProfile.Elapsed * TIMEOUT_COEFFICIENT` (engine/executor.go:101),
    and Elapsed is the wall time of gremlins' OWN coverage run — which it invokes
    as `go test -cover -coverprofile <file> <pkg>` with NO -count=1
    (coverage/coverage.go:145-157). That is eligible for Go's test result cache.

    Measured on internal/gateway, back to back:

        run 1   9.286s          -> deadline ~336s   suite 9.3s   fine
        run 2   (cached) 0.27s  -> deadline ~8.1s   suite 9.3s   ALL time out

    A 41x collapse, landing the deadline BELOW the suite's own runtime. That is
    the 55-of-78 gateway timeouts, and the internal/mcp history recorded at
    TIMEOUT_COEFFICIENT (58 timeouts, then 1, then 58 over identical code) with
    no appeal to machine load at all.

    Failure here is NOT fatal. A gate that refuses to run because it could not
    clear a cache has turned a slow measurement into no measurement; the guard
    that catches a collapsed deadline is the majority-timeout check, which stays.
    """
    try:
        subprocess.run(clear_test_cache_args(), capture_output=True, check=False)
    except OSError:
        pass


def default_runner(pkg):
    proc = subprocess.run(gremlins_args(pkg), capture_output=True, text=True)
    return proc.stdout + proc.stderr


def _disk_targets():
    """The directories a mutation run consumes: the tree copies, and the cache."""
    targets = [tempfile.gettempdir()]
    cache = go_cache()
    if cache:
        targets.append(cache)
    return targets


def _free(path):
    try:
        return shutil.disk_usage(path).free
    except OSError:
        # An unreadable path must not silently lower the floor to nothing.
        return float("inf")


def run(equivalents_path, packages=PACKAGES, runner=default_runner,
        out=sys.stdout, err=sys.stderr, root=".", free_bytes=None,
        cache_bytes=None, prepare=clear_test_cache):
    # BEFORE anything is spent. Discovering a full disk as a corrupt verdict is
    # the failure this whole file's error handling is about; discovering it as
    # a number, up front, costs nothing.
    #
    # $TMPDIR rather than the repo: that is where gremlins copies the tree per
    # mutant, and on this platform it is a different volume from the source in
    # principle even though it is not in practice.
    if free_bytes is None:
        # BOTH volumes, whichever is tighter. $TMPDIR takes the per-mutant tree
        # copies; GOCACHE takes the retained growth, and they are not always
        # the same filesystem — a systemd /tmp is a RAM-backed tmpfs, where
        # reading only $TMPDIR would refuse forever on a machine with hundreds
        # of free gigabytes AND `go clean -cache` could not move the number by
        # a byte. The inverse is worse: /tmp looks roomy while the cache volume
        # is full, and the pre-flight waves through the run this exists to stop.
        free_bytes = min(_free(d) for d in _disk_targets())
    if free_bytes < MIN_FREE_BYTES:
        print(f"check:mutation: {free_bytes / 1024**3:.1f} GiB free on "
              f"{tempfile.gettempdir()}, and this needs at least "
              f"{MIN_FREE_BYTES / 1024**3:.0f} GiB. gremlins copies the tree per mutant and "
              f"every mutant is a distinct binary, so a full gate run costs about 7.6 GiB and "
              f"leaves GOCACHE ~4 GiB larger — unbounded across runs, measured at 82 GiB. "
              f"Run `go clean -cache` and try again. REFUSING RATHER THAN RUNNING: out of space, "
              f"gremlins fails to apply or restore mutations, scores the result anyway, prints a "
              f"normal summary and exits 0 — a wrong answer that looks exactly like a right one.",
              file=err)
        return 1

    # PAST the floor, so this run is going ahead. The floor is a cliff; this is
    # the ramp, while there is still room to act.
    if free_bytes < CACHE_WARN_FREE_BYTES:
        if cache_bytes is None:
            cache_bytes = cache_size(go_cache())
        recover = ("" if cache_bytes is None else
                   f" `go clean -cache` would recover about "
                   f"{cache_bytes / 1024**3:.1f} GiB of that.")
        print(f"check:mutation: {free_bytes / 1024**3:.1f} GiB free, heading for the "
              f"{MIN_FREE_BYTES / 1024**3:.0f} GiB floor that refuses to run at all. "
              f"Running anyway — this is a warning, not a floor. Every mutant is a "
              f"distinct binary, so the build cache grows without bound across runs; "
              f"measured at 82 GiB on a 228 GiB volume, and once the volume actually "
              f"fills, Bash stops working, so the failure does not even present as a "
              f"gate failure.{recover}",
              file=err)

    # AFTER the floor, so a refused run does not also throw away the test cache
    # somebody else's next `go test` would have used — and BEFORE the first
    # package, because clearing it later leaves that package judged against a
    # collapsed deadline, which is the failure rather than a smaller version.
    prepare()

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
