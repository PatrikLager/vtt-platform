#!/usr/bin/env python3
"""Enforce a per-package coverage ratchet over one merged Go coverage profile.

Why this exists
---------------
ckeletin-go's standard is ">=85% test coverage" as a machine-checkable gate,
and Patrik's ruling (2026-07-27): "The coverage 85% is a minimum. The higher
the better." A flat 85% floor would permit regression — internal/engine at
97.7% could rot all the way to 85% while the gate stayed green the whole way,
the exact drift a gate exists to stop. So thresholds RATCHET: each package is
held to what it has already earned, recorded in coverage-thresholds.txt.

What this script deliberately does NOT do
-----------------------------------------
It does not merge profiles. `go tool covdata merge` does that natively, and
check:coverage uses it. An earlier version hand-rolled the merge here — a
block parser, a count-summing union, a covermode agreement check — and four
review rounds found THREE separate "passes when it should fail" defects in
that code and its wiring, against one defect total in the eight Go test files
this change set exists to add. The layer that kept producing enforcement bugs
was the layer Go already implements. Verified equivalent before deletion: the
native pipeline reproduces all thirteen package numbers exactly, cmd/vtt's
85.2% included.

`go tool covdata percent` is deliberately not used either — its output is not
reliably one-package-per-line (a zero-statement package emits its name with no
percentage and the next package's line runs on behind it), so parsing it would
trade a solved problem for a fragile one. The textfmt profile is the
well-defined interface.

This script IS the gate, so it fails closed in four directions a naive version
would not — each one a defect a review demonstrated, not a hypothetical:
  * a threshold naming a package that was not measured is an ERROR, not a
    silent default — one typo ("engnie") otherwise drops a 97.5 floor to 85.0;
  * every measured package must have a recorded floor, so thresholds and the
    measured set are equal BY CONSTRUCTION and narrowing what gets measured
    cannot quietly shrink the gate to whatever remains;
  * a missing or empty thresholds file is an ERROR, not a fallback to the
    default floor for all thirteen packages at once;
  * a package that SHOULD have been measured but produced no coverage data at
    all is an ERROR — `go test -cover -args -test.gocoverdir` emits nothing for
    a package with no _test.go files, so an untested package is invisible to
    both set-equality directions (it is in neither `measured` nor
    `thresholds`); the expected list closes that;
  * a floor BELOW the 85% minimum is an ERROR — otherwise the standard the
    whole script cites is the one number nothing checks, and any floor could
    be weakened by editing a digit.

Its own tests are tools/check_coverage_test.py, run by check:coverage before
the gate itself.
"""

import math
import os
import sys
from collections import defaultdict

FLOOR = 85.0

# Excluded from the gate, with the reason each is exempt.
EXCLUDE_PREFIXES = (
    "github.com/PatrikLager/vtt-platform/contract/gen/",     # generated code
    "github.com/PatrikLager/vtt-platform/contract-spike/",   # frozen ADR-007 evidence, read-only
)

# How far above its floor a package must sit before we suggest raising it.
# Wide enough not to nag about run-to-run noise (harness has a measured 0.79pp
# spread), narrow enough that real gains get ratcheted in.
SUGGEST_RAISE_BAND = 1.0


class ProfileError(Exception):
    pass


def parse_profile(path):
    """Return {package: (covered_statements, total_statements)}.

    Profile lines are `name.go:line.col,line.col nstmt count`, where `name.go`
    is an import-path-qualified filename that MAY CONTAIN SPACES — hence rsplit
    on the two trailing numeric fields rather than a naive split.
    """
    covered = defaultdict(int)
    total = defaultdict(int)
    with open(path) as fh:
        for lineno, raw in enumerate(fh, 1):
            line = raw.strip()
            if not line or line.startswith("mode:"):
                continue
            parts = line.rsplit(" ", 2)
            if len(parts) != 3:
                raise ProfileError(f"{path}:{lineno}: malformed profile line: {line!r}")
            location, nstmt_s, count_s = parts
            try:
                nstmt, count = int(nstmt_s), int(count_s)
            except ValueError as exc:
                # Reached by any line that happens to split into three fields
                # but is not a profile line — same corruption class as a wrong
                # field count, so reported the same way. Silently skipping
                # either would drop statements from a package's total and
                # INFLATE its percentage.
                raise ProfileError(
                    f"{path}:{lineno}: malformed profile line: {line!r} ({exc})") from exc
            pkg = location.rsplit("/", 1)[0]  # strip the file, leaving the import path
            if pkg.startswith(EXCLUDE_PREFIXES):
                continue
            total[pkg] += nstmt
            if count > 0:
                covered[pkg] += nstmt
    return {p: (covered[p], total[p]) for p in total if total[p] > 0}


def read_expected(path):
    """Packages that MUST appear in the profile, one import path per line.

    Produced by tools/list-measured-packages.sh. This is NOT redundant with the
    thresholds file: set equality is between thresholds and what was MEASURED,
    and a package with no _test.go files is never measured at all under
    `go test -cover -args -test.gocoverdir` (it has no test binary, so it emits
    no covdata). Such a package appears in neither set, so neither fatal fires,
    and an untested package ships with the gate reporting success.

    An earlier version of this script had this guard, and it was removed as
    unreachable-and-tested code — correctly, for the pipeline of the time,
    where untested packages showed up at 0% and the missing-floor fatal caught
    them. Switching to covdata invalidated that premise and reopened the hole.
    """
    if not os.path.exists(path):
        raise ProfileError(
            f"expected-packages file {path} does not exist — a package with no tests "
            f"would be invisible to the gate")
    expected = {line.strip() for line in open(path) if line.strip()}
    if not expected:
        raise ProfileError(f"expected-packages file {path} is empty")
    return expected


def read_thresholds(path):
    """Parse the ratchet file. Missing, empty, or below-minimum is FATAL.

    Returning {} for a missing file would silently drop every recorded floor to
    FLOOR and pass: one typo loses one floor (the stale-key check catches
    that), but a deleted or mistyped PATH loses all of them with nothing to
    notice. The ratchet is the gate's memory; no memory is not a green build.
    """
    if not os.path.exists(path):
        raise ProfileError(
            f"thresholds file {path} does not exist — every ratcheted floor would "
            f"silently fall back to {FLOOR:.0f}%")

    thresholds = {}
    errors = []
    with open(path) as fh:
        for lineno, raw in enumerate(fh, 1):
            line = raw.split("#", 1)[0].strip()
            if not line:
                continue
            try:
                pkg, value = line.rsplit(None, 1)
                floor = float(value)
            except ValueError:
                errors.append(f"{path}:{lineno}: malformed threshold line: {line!r}")
                continue
            # isfinite BEFORE the bound: float() accepts "nan", "inf" and
            # "1e400", and `nan < FLOOR` / `inf < FLOOR` are both False, so
            # they slip past a bare comparison. Neither can make the gate PASS
            # — nothing is `>= nan` or `>= inf` — but the message became
            # "below its floor of nan%", sending a reader hunting a coverage
            # regression that does not exist. In the gate whose job is
            # legibility, that is its own defect.
            if not math.isfinite(floor) or floor < FLOOR:
                errors.append(
                    f"{path}:{lineno}: floor {value!r} for {pkg} is not a real percentage at or "
                    f"above the {FLOOR:.0f}% minimum — raise the package's coverage instead. A "
                    f"genuine exception is a reviewed decision: change FLOOR here and say why "
                    f"in the ledger")
                continue
            thresholds[pkg] = floor

    # Report every bad line at once; the set-equality fatals below already
    # batch, and three bad floors should not cost three round trips.
    if errors:
        raise ProfileError("\n  ".join(["bad threshold lines:"] + errors))
    if not thresholds:
        raise ProfileError(
            f"thresholds file {path} declares no floors — every package would fall "
            f"back to {FLOOR:.0f}%")
    return thresholds


def run(thresholds_path, profile_path, expected_path=None, out=sys.stdout, err=sys.stderr):
    try:
        thresholds = read_thresholds(thresholds_path)
        expected = read_expected(expected_path) if expected_path else set()
        if not os.path.exists(profile_path) or os.path.getsize(profile_path) == 0:
            raise ProfileError(f"coverage profile {profile_path} is missing or empty")
        measured = parse_profile(profile_path)
    except ProfileError as exc:
        print(f"check:coverage: {exc}", file=err)
        return 1

    if not measured:
        print("check:coverage: profile contains no measurable packages", file=err)
        return 1

    fatal = []
    # A package that should have been measured but emitted nothing. Checked
    # FIRST because it is the only one of the three that can see a package
    # missing from BOTH sets — the others compare thresholds against measured,
    # and this catches what never reached either.
    for pkg in sorted(expected - set(measured)):
        fatal.append(
            f"{pkg} produced no coverage data — a package with no _test.go files emits no "
            f"covdata at all, so it is invisible to the floors. Add tests for it (ADR-009: "
            f"tests first), or remove it")

    # A threshold naming a package we did not measure is stale or misspelled;
    # defaulting it to FLOOR would silently discard a ratcheted floor.
    for pkg in sorted(set(thresholds) - set(measured)):
        fatal.append(
            f"threshold names {pkg}, which was not measured — the package was renamed, "
            f"moved, or dropped from the measured set, and its {thresholds[pkg]:.1f}% "
            f"floor is not being enforced")
    # And every measured package must have a floor. This is what makes the two
    # sets equal by construction: without it a package with no entry sits
    # outside the expectation set, so nothing notices if it stops being
    # measured.
    for pkg in sorted(set(measured) - set(thresholds)):
        cov, tot = measured[pkg]
        fatal.append(
            f"{pkg} has no floor in {thresholds_path} — measured {100.0 * cov / tot:.1f}%; "
            f"add a line for it (>= {FLOOR:.0f}) so the ratchet can hold it")

    if fatal:
        for msg in fatal:
            print(f"check:coverage: {msg}", file=err)
        return 1

    rows = []
    for pkg in sorted(measured):
        cov, tot = measured[pkg]
        pct = 100.0 * cov / tot
        # Not .get(pkg, FLOOR): the missing-floor fatal above guarantees the
        # key, so a default would be dead code implying an impossible fallback.
        want = thresholds[pkg]
        rows.append((pkg, pct, want, pct + 1e-9 >= want))

    width = max(len(p) for p, _, _, _ in rows)
    for pkg, pct, want, ok in rows:
        print(f"{'ok ' if ok else 'FAIL'} {pkg:<{width}}  {pct:6.1f}%  (min {want:.1f}%)", file=out)

    failures = [(p, pct, want) for p, pct, want, ok in rows if not ok]
    if failures:
        print("", file=err)
        for pkg, pct, want in failures:
            print(f"check:coverage: {pkg} at {pct:.1f}%, below its floor of {want:.1f}%", file=err)
        print(f"\nRaise coverage, or — if a floor is genuinely wrong — change it in "
              f"{thresholds_path} as a reviewed decision (CLAUDE.md rule 2).", file=err)
        return 1

    # The ratchet only ratchets if someone turns it. Say so, rather than
    # leaving earned coverage un-banked until it quietly rots back down.
    raises = [(p, pct, want) for p, pct, want, _ in rows if pct - want > SUGGEST_RAISE_BAND]
    if raises:
        print(f"\ncheck:coverage: {len(raises)} package(s) now clear their floor by more than "
              f"{SUGGEST_RAISE_BAND:.1f}pp — consider ratcheting in {thresholds_path}:", file=out)
        for pkg, pct, want in raises:
            print(f"  {pkg}  {want:.1f} -> {pct - 0.3:.1f}   (measured {pct:.1f}%)", file=out)

    print(f"\ncheck:coverage: {len(rows)} packages, all at or above their floors.", file=out)
    return 0


def main(argv):
    if len(argv) != 4:
        print("usage: check-coverage.py <thresholds-file> <merged-profile> <expected-packages>",
              file=sys.stderr)
        return 2
    return run(argv[1], argv[2], argv[3])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
