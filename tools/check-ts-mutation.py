#!/usr/bin/env python3
"""Fail on any surviving TypeScript mutant not adjudicated equivalent.

The Go sibling is tools/check-mutation.py and the reasoning is identical —
read its header first. The short version: gate on ZERO UNADJUDICATED
SURVIVORS, not on a mutation-score percentage, because a score is a ratio and
enough newly-killed mutants mask a new survivor.

Why mutation testing for the client at all
------------------------------------------
Because coverage said the client was fine and it was not. undo.ts sat at 100%
line coverage and scored 78.75% mutation, and the survivors were on the
`>= from` / `<= to` comparisons — the INCLUSIVE [from,to] range that is the
entire semantic of retraction. Every line ran; nothing checked the boundary.
That is precisely the gap a coverage ratchet cannot see, and it is the reason
this gate exists alongside check:ts-coverage rather than instead of it.

Why Stryker's command runner
----------------------------
Stryker has no bun test-runner plugin, so it runs `bun test client/test` as an
opaque command and reads the exit code. The cost is no per-test filtering:
every mutant runs the whole suite. Measured, that is fine — the suite is
~700ms, and 1667 mutants complete in minutes. `coverageAnalysis: "off"` is
mandatory with the command runner, not a tuning choice.

This file parses JSON with the json module and compares two sets. It does not
implement mutation testing or interpret TypeScript. That boundary is the same
one check-mutation.py draws, for the same reason recorded there: across five
review rounds of the coverage gate, 8 of 9 defects were in hand-rolled
enforcement code.
"""

import json
import sys
from pathlib import Path

# Statuses Stryker reports. Grouped by what they MEAN for the gate rather than
# by name, because the names do not map onto the decision.
KILLED = "Killed"
SURVIVED = "Survived"
TIMEOUT = "Timeout"
NO_COVERAGE = "NoCoverage"
# A mutant that could not be built or blew up the runner is not a measurement
# in either direction — gremlins calls this "not viable".
NOT_VIABLE = ("CompileError", "RuntimeError", "Ignored")

# Timed-out mutants count as detections (Patrik's ruling, 2026-07-28) but are
# NOT reliably detections, and check-mutation.py records the counterexample
# that disproves the tempting claim that they are. So: every one is printed,
# and a run that times out on a majority of mutants is rejected as an unusable
# measurement rather than scored as a good one.
MAX_TIMEOUT_FRACTION = 0.50


class EquivalentsError(Exception):
    pass


def read_equivalents(path):
    """{(file, "line:col", mutator, replacement): reason}.

    A header line plus an indented reason. The reason is REQUIRED. An
    equivalence claim asserts that NO test can ever kill the mutant, so it
    forecloses writing one; on the Go side two of four such claims were wrong,
    and a bare entry is how that happens quietly.
    """
    entries = {}
    pending = None
    for lineno, raw in enumerate(Path(path).read_text().splitlines(), 1):
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if raw[:1].isspace():
            if pending is None:
                raise EquivalentsError(f"{path}:{lineno}: reason with no entry above it")
            entries[pending] = line.strip()
            pending = None
            continue
        if pending is not None:
            raise EquivalentsError(f"{path}:{pending_line}: entry with no reason under it")
        # The REPLACEMENT is part of the key, and is the rest of the line
        # because it can contain spaces (`a.id <= b.id`).
        #
        # Without it, two mutants at one location with one mutator — Stryker
        # emits `-> true` AND `-> false` for the same ConditionalExpression —
        # collapse to a single key, and adjudicating either silently excuses
        # the other. Found by this file's own duplicate check on
        # client/src/player.ts:20:51, where the two are genuinely different
        # claims: forcing that branch to `true` returns what it would have
        # returned anyway, while forcing it to `false` does not.
        parts = line.split(maxsplit=3)
        if len(parts) != 4:
            raise EquivalentsError(
                f"{path}:{lineno}: want '<file> <line>:<col> <MUTATOR> <replacement>', "
                f"got {line.strip()!r}")
        key = (parts[0], parts[1], parts[2], parts[3])
        if key in entries:
            raise EquivalentsError(f"{path}:{lineno}: {key} listed twice")
        pending, pending_line = key, lineno
    if pending is not None:
        raise EquivalentsError(f"{path}:{pending_line}: entry with no reason under it")
    return entries


def mutants(report):
    """Yield (file, "line:col", mutator, status) for every mutant."""
    for path, entry in sorted(report.get("files", {}).items()):
        for m in entry.get("mutants", []):
            start = m.get("location", {}).get("start", {})
            where = f"{start.get('line', '?')}:{start.get('column', '?')}"
            # Replacement is normalised to one line: it is a key component, and
            # Stryker emits multi-line replacements for block mutants.
            replacement = " ".join(str(m.get("replacement", "")).split())
            yield path, where, m.get("mutatorName", "?"), replacement, m.get("status", "?")


def check(report, equivalents, out=sys.stdout, err=sys.stderr):
    survived, timed_out, killed, no_coverage, not_viable = [], [], 0, [], 0
    for path, where, mutator, replacement, status in mutants(report):
        if status == KILLED:
            killed += 1
        elif status == SURVIVED:
            survived.append((path, where, mutator, replacement))
        elif status == TIMEOUT:
            timed_out.append((path, where, mutator, replacement))
        elif status == NO_COVERAGE:
            no_coverage.append((path, where, mutator, replacement))
        elif status in NOT_VIABLE:
            not_viable += 1
        else:
            print(f"check:ts-mutation: unknown status {status!r} at {path}:{where}. "
                  f"Refusing to guess whether that is a detection.", file=err)
            return 1

    fail = False

    # Not covered means NO test reaches the code at all. With the command
    # runner that should be impossible — every mutant runs the whole suite —
    # so its appearance means the run was not configured the way this gate
    # assumes, and the numbers below do not mean what they say.
    for path, where, mutator, _ in no_coverage:
        print(f"check:ts-mutation: {path}:{where} {mutator} reported NoCoverage, which "
              f"the command runner should never produce. The run is misconfigured.", file=err)
        fail = True

    measured = killed + len(survived) + len(timed_out)
    if measured == 0:
        print("check:ts-mutation: no mutants were measured. Refusing to report success "
              "over nothing.", file=err)
        return 1

    # Printed, always. These are the mutants the run did NOT actually evaluate;
    # discarding them silently is how a survivor disappears.
    for path, where, mutator, _ in timed_out:
        print(f"    timed out: {path}:{where} {mutator} — not evaluated; if this "
              f"persists, make the test that blocks under it fail fast", file=out)

    fraction = len(timed_out) / measured
    if fraction > MAX_TIMEOUT_FRACTION:
        print(f"check:ts-mutation: {len(timed_out)} of {measured} mutants timed out "
              f"({fraction:.0%}). Counting those as kills would score a broken run as a "
              f"good one — fix what hangs before trusting this.", file=err)
        fail = True

    # An edit ABOVE an adjudicated mutant shifts its line, and the gate used to
    # report that as two unrelated failures — a stale entry here, an
    # unadjudicated survivor there — with nothing saying they were the same
    # mutant. The obvious response to "no longer survives, remove it" is then
    # to DELETE a reasoned adjudication and leave the survivor unexplained.
    # Measured: that happened four times in one day, across both gates.
    #
    # A move is only claimed when the pairing is UNAMBIGUOUS: same file, same
    # mutator, same replacement text, exactly one stale entry and exactly one
    # unadjudicated survivor to match it. Anything less and both are reported
    # the old way — guessing would re-key an adjudication onto a mutant nobody
    # judged, which is the failure this gate exists to prevent, arrived at by a
    # convenience.
    live = set(survived)
    stale_keys = sorted(set(equivalents) - live)
    unadjudicated = [m for m in survived if m not in equivalents]

    def _sig(key):
        return (key[0], key[2], key[3])  # file, mutator, replacement

    moved = {}
    for key in stale_keys:
        cands = [m for m in unadjudicated if _sig(m) == _sig(key)]
        rivals = [k for k in stale_keys if _sig(k) == _sig(key)]
        if len(cands) == 1 and len(rivals) == 1:
            moved[key] = cands[0]

    for was, now in sorted(moved.items()):
        print(f"check:ts-mutation: ADJUDICATION MOVED {was[0]} {was[2]} (-> {was[3]}): "
              f"{was[1]} -> {now[1]}. Same mutator and same mutation, so an edit shifted "
              f"it. RE-KEY the entry to the new line; do NOT delete it, or its reasoning "
              f"is lost and the survivor is left unexplained.", file=err)
        fail = True

    unadjudicated = [m for m in unadjudicated if m not in moved.values()]
    stale_keys = [k for k in stale_keys if k not in moved]
    for path, where, mutator, replacement in unadjudicated:
        print(f"check:ts-mutation: SURVIVED {mutator} at {path}:{where} (-> {replacement}) — no "
              f"test distinguishes the mutated code. Kill it, or adjudicate it equivalent "
              f"with a stated observable.", file=err)
        fail = True

    # A stale adjudication excuses whatever later lands at that location.
    for key in stale_keys:
        print(f"check:ts-mutation: {key[0]}:{key[1]} {key[2]} (-> {key[3]}) is adjudicated equivalent "
              f"but no longer survives — it was killed, moved, or the line shifted. "
              f"Remove the entry rather than let it pre-approve a future mutant.", file=err)
        fail = True

    if fail:
        return 1
    print(f"check:ts-mutation: {measured} mutants, {killed} killed, "
          f"{len(timed_out)} timed out, {len(survived)} adjudicated equivalent, "
          f"zero unadjudicated survivors.", file=out)
    return 0


def main(argv):
    root = Path(__file__).resolve().parent.parent
    report_path = root / "reports" / "mutation" / "mutation.json"
    if not report_path.exists():
        print(f"check:ts-mutation: {report_path} not found — run stryker first "
              f"(task check:ts-mutation does).", file=sys.stderr)
        return 1
    try:
        equivalents = read_equivalents(argv[1])
    except (EquivalentsError, OSError) as e:
        print(f"check:ts-mutation: {e}", file=sys.stderr)
        return 1
    return check(json.loads(report_path.read_text()), equivalents)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: check-ts-mutation.py <equivalents-file>", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv))
