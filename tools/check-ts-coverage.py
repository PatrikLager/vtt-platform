#!/usr/bin/env python3
"""Enforce a per-FILE coverage ratchet over the TypeScript client.

The Go side's equivalent is tools/check-coverage.py, and this deliberately
copies its shape — including every direction it fails closed. Read that file's
header first; the reasoning is the same and is not repeated here.

What is different about the TypeScript side
-------------------------------------------
bun reports coverage ONLY for files a test imported. Files nothing imports are
not 0% — they are absent, which is worse, because they leave the denominator
too. When this gate was written, app.ts, view/dm.ts and view/spectator.ts —
660 lines, 30% of the client — were invisible for exactly that reason, and the
headline read 89.78% over the remaining two thirds. A global-threshold gate
would have been green the whole time.

So the file list is taken from the SOURCE TREE, not from the report, and a
source file missing from the report is an error. client/test/all-modules.test.ts
imports the tree to keep that honest; this gate is what notices if it stops.

Its own tests are tools/check_ts_coverage_test.py.
"""

import math
import shutil
import subprocess
import sys
import time
from pathlib import Path

FLOOR = 85.0

# Clearing a floor by more than this suggests the floor is stale.
RAISE_BAND = 1.0

# Excluded from the gate, with the reason each is exempt. Adding to this list
# is a reviewed decision: an exemption is a permanent hole, not a TODO.
EXCLUDED = {
    "client/src/state.ts": (
        "80 lines of type declarations, which erase at runtime, leaving THREE "
        "executable lines: newState's declaration, its body, and a class. Its "
        "declaration line reports 0 hits while its own body reports 95 in the "
        "same lcov record — an instrumentation artifact, not untested code "
        "(verified: 60+ other function declarations in this tree do report "
        "hits, so it is not a blanket bun behaviour). That single unreachable "
        "line caps the file at 66.7% forever, and one line moving the number "
        "33pp makes any floor here meaningless."
    ),
}

# Generated and spike code: measured, never gated. protoc-gen-es output's
# coverage reflects which messages the tests happen to construct.
# Test helpers are imported by tests, so they appear in the report; gating a
# helper's coverage measures the tests, not the product.
# The TRAILING SLASH on each entry is load-bearing, and is the same lesson as
# test:unit's `(/|$)` grep boundary: a bare "client/test" prefix would also
# swallow a future client/testdata or client/tests, dropping it out of the
# gate while everything still reported green. Keep the slash.
UNGATED_PREFIXES = ("contract/gen/", "contract-spike/", "client/test/")

SRC_ROOT = Path("client") / "src"


def parse_lcov(text):
    """file -> (covered, total) executable lines."""
    out = {}
    current, covered, total = None, 0, 0
    for line in text.splitlines():
        if line.startswith("SF:"):
            current, covered, total = line[3:].strip(), 0, 0
        elif line.startswith("DA:") and current is not None:
            # DA:<line>,<hits>[,<checksum>] — the checksum field is standard
            # lcov and bun does not emit it today. rpartition read the LAST
            # field, so a checksum would have been parsed as the hit count
            # (or raised a bare traceback). Index, do not guess.
            fields = line[3:].split(",")
            try:
                hits = int(fields[1])
            except (IndexError, ValueError):
                raise ValueError(
                    f"malformed coverage record {line!r} for {current}. Skipping it "
                    f"would inflate that file's percentage, so this refuses instead.")
            total += 1
            covered += 1 if hits > 0 else 0
        elif line.startswith("end_of_record") and current is not None:
            _merge(out, current, covered, total)
            current = None
    if current is not None:
        _merge(out, current, covered, total)
    return out


def _merge(out, path, covered, total):
    """Reject a duplicate record rather than guess how to combine it.

    bun 1.3.10 emits one record per resolved path, so this is not reachable
    today. Last-wins was the original behaviour and is the dangerous one: a
    0/10 record followed by a 10/10 record for one file reports 100%.

    Rejecting beats merging because the right merge is not knowable from here.
    Summing (what check-coverage.py does for Go packages, where each record is
    a distinct file) would double-count a file measured twice; a per-line
    union is what lcov actually means, and this parser has already discarded
    the line numbers by the time it would need them. So: refuse, loudly, and
    let whoever makes it reachable decide with the facts in hand.
    """
    if path in out:
        raise ValueError(
            f"duplicate coverage record for {path}. This parser cannot know "
            f"whether to sum or union them, and last-wins would report the "
            f"second record's percentage as if it were the whole file.")
    out[path] = (covered, total)


def parse_thresholds(text, path="thresholds"):
    out = {}
    for n, raw in enumerate(text.splitlines(), 1):
        line = raw.split("#", 1)[0].strip()
        if not line:
            continue
        parts = line.split()
        if len(parts) != 2:
            raise ValueError(f"{path}:{n}: want '<file> <percent>', got {raw!r}")
        try:
            pct = float(parts[1])
        except ValueError:
            raise ValueError(f"{path}:{n}: {parts[1]!r} is not a number")
        # float() accepts "nan", "inf" and "1e400". nan is the dangerous one
        # HERE in a way it is not on the Go side: check-coverage.py records a
        # failure when `pct >= want` is False, so a nan floor fails loudly,
        # while this script records one when `pct < want` is True, and
        # `x < nan` is False for every x. Porting the shape without this
        # guard turned nan from a loud failure into a silent exemption —
        # below-minimum passes it, and the file can then never fail.
        if not math.isfinite(pct):
            raise ValueError(f"{path}:{n}: {parts[1]!r} is not a finite number")
        if parts[0] in out:
            raise ValueError(f"{path}:{n}: {parts[0]} listed twice")
        out[parts[0]] = pct
    if not out:
        raise ValueError(f"{path}: no thresholds — refusing to gate nothing")
    return out


# Kept in step with walk() in client/test/all-modules.test.ts BY HAND, because
# one is Python and the other TypeScript. A review found three divergences in
# the first version, each of which hides a file from the gate while the
# success line still reads "N files at or above their floors":
#
#   .d.ts        — excluded here, INCLUDED by walk(), which then tried to
#                  import a declaration file.
#   .tsx/.mts    — matched by NEITHER, so adopting JSX for one view put it in
#                  neither `expected` nor (unimported) `measured`. Invisible.
#   symlinked
#   directories  — rglob does not descend into them; walk()'s statSync
#                  follows, putting files in `measured` that are not expected.
#
# test_the_walk_matches_the_barrel_test in check_ts_coverage_test.py pins the
# extension rule on this side.
SOURCE_SUFFIXES = (".ts", ".tsx", ".mts", ".cts")


def expected_files(root):
    """Every source module under client/src. The set that MUST be measured."""
    out = set()
    base = root / SRC_ROOT
    for p in base.rglob("*"):
        if not p.is_file() or p.is_symlink():
            continue
        if p.name.endswith(".d.ts") or p.suffix not in SOURCE_SUFFIXES:
            continue
        out.add(str(p.relative_to(root)).replace("\\", "/"))
    return out


def check(lcov_text, thresholds_text, expected, err, out=sys.stdout):
    try:
        measured = {
            f: v for f, v in parse_lcov(lcov_text).items()
            if not f.startswith(UNGATED_PREFIXES)
        }
    except ValueError as e:
        print(f"check:ts-coverage: {e}", file=err)
        return 1
    try:
        floors = parse_thresholds(thresholds_text)
    except ValueError as e:
        print(f"check:ts-coverage: {e}", file=err)
        return 1

    # M3: an empty expected set turns off the one check that distinguishes
    # this gate from a bunfig.toml coverageThreshold line. It goes empty the
    # day client/src moves, and the obvious fix for the resulting path errors
    # (rewrite the thresholds file) leaves the gate green and blind.
    if not expected:
        print(f"check:ts-coverage: no source files found under {SRC_ROOT} — either the "
              f"tree moved and SRC_ROOT is stale, or the walk is broken. Refusing to "
              f"report success over nothing.", file=err)
        return 1

    fail = False

    def bad(msg):
        nonlocal fail
        print(f"check:ts-coverage: {msg}", file=err)
        fail = True

    # A file cannot be both exempt and gated. The exclusion wins silently, so
    # the floor would sit in the thresholds file looking enforced while
    # nothing ever consulted it — the same "green gate checking nothing" shape
    # this script exists to prevent, one level up. Found by its own tests.
    for f in sorted(set(floors) & set(EXCLUDED)):
        bad(f"{f} is EXCLUDED in the checker AND carries a floor in the thresholds "
            f"file. The exclusion wins, so that floor is decoration. Remove one.")

    # An exemption is the only hole in an otherwise total gate, so it gets the
    # same staleness check the floors get. state.ts's recorded reason is about
    # a specific file ("THREE executable lines"); move the types out and put
    # real logic at that path and the exemption silently covers the new file,
    # while the success line still reads "(1 excluded)".
    for f in sorted(set(EXCLUDED) - expected):
        bad(f"{f} is EXCLUDED but no longer exists in the source tree. A stale "
            f"exemption pre-approves whatever later takes that path — remove it.")
    for f in sorted((set(EXCLUDED) & expected) - set(measured)):
        bad(f"{f} is EXCLUDED but absent from the coverage report. Its exemption is "
            f"recorded against what the file measures, so it must still be measured "
            f"for that reason to be checkable.")

    # A floor below the stated minimum would make 85% the one number nothing
    # checks — any gate could then be weakened by editing a digit.
    for f, pct in sorted(floors.items()):
        if pct < FLOOR:
            bad(f"{f} has a floor of {pct:.1f}%, below the {FLOOR:.0f}% minimum. "
                f"Raise the coverage, or add the file to EXCLUDED with a reason.")

    # A source file absent from the report was never imported by any test.
    for f in sorted(expected - set(measured) - set(EXCLUDED)):
        bad(f"{f} is absent from the coverage report — no test imports it, so its "
            f"lines are excluded from every percentage rather than counted as 0. "
            f"Import it (client/test/all-modules.test.ts imports the tree) instead "
            f"of lowering a threshold.")

    # Set equality both ways, so narrowing what is measured cannot quietly
    # shrink the gate to whatever remains, and a typo cannot drop a floor.
    for f in sorted(set(measured) - set(floors) - set(EXCLUDED)):
        bad(f"{f} is measured but has no recorded floor. Add one at or above "
            f"{FLOOR:.0f}%.")
    for f in sorted(set(floors) - set(measured)):
        bad(f"a floor is recorded for {f}, which was not measured. A stale entry "
            f"pre-approves whatever later takes that path — remove it.")

    gated = 0
    for f, (covered, total) in sorted(measured.items()):
        if f in EXCLUDED or f not in floors:
            continue
        if total == 0:
            bad(f"{f} reports zero executable lines; a percentage over nothing "
                f"cannot gate anything.")
            continue
        gated += 1
        pct = 100.0 * covered / total
        if pct + 1e-9 < floors[f]:
            bad(f"{f} at {pct:.2f}%, below its floor of {floors[f]:.2f}% "
                f"({covered}/{total} lines)")

    if fail:
        return 1

    # The ratchet only ratchets if someone turns it (check-coverage.py:263).
    # A file that improves and is never banked can rot straight back with the
    # gate green the whole way -- the drift this exists to stop.
    for f, (covered, total) in sorted(measured.items()):
        if f in EXCLUDED or f not in floors:
            continue
        pct = 100.0 * covered / total
        note = ""
        if pct - floors[f] > RAISE_BAND:
            note = f"   <-- clears by {pct - floors[f]:.2f}pp; raise to {math.floor(pct * 100) / 100:.2f}"
        print(f"ok  {f:<32} {pct:6.2f}%  (min {floors[f]:.2f}%){note}", file=out)

    # Counts, not just a pass line: an empty expected set, a stale exemption
    # or a nan floor all used to print output byte-identical to a healthy run.
    print(f"check:ts-coverage: {len(expected)} source files expected, {gated} gated, "
          f"{len(EXCLUDED)} excluded.", file=out)
    return 0


def main(argv):
    root = Path(__file__).resolve().parent.parent
    covdir = root / ".tscov"
    # check:coverage builds its profile in a fresh mktemp -d and so CANNOT
    # read data it did not just produce. This directory persists, so clear it
    # and then require the report to be newer than the run: otherwise any
    # future bun that writes elsewhere (a moved --coverage-dir, a bunfig
    # [test] key, a wrapper on PATH) makes the gate grade a stale file.
    shutil.rmtree(covdir, ignore_errors=True)
    started = time.time()
    proc = subprocess.run(
        ["bun", "test", "client/test", "contract", "contract-spike",
         "--coverage", "--coverage-reporter=lcov", f"--coverage-dir={covdir}"],
        cwd=root, capture_output=True, text=True,
    )
    lcov = covdir / "lcov.info"
    if proc.returncode != 0 or not lcov.exists():
        sys.stderr.write(proc.stdout + proc.stderr)
        print("check:ts-coverage: the test run produced no coverage report", file=sys.stderr)
        return 1
    if lcov.stat().st_mtime < started - 1:
        print(f"check:ts-coverage: {lcov} predates this run — it is a leftover report, "
              f"not this suite's. Refusing to grade it.", file=sys.stderr)
        return 1
    return check(
        lcov.read_text(),
        Path(argv[1]).read_text(),
        expected_files(root),
        sys.stderr,
    )


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: check-ts-coverage.py <thresholds-file>", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv))
