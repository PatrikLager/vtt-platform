#!/usr/bin/env python3
"""Print a hash of everything that can change the TS mutation REPORT.

WHY THIS EXISTS. check:ts-mutation welds two operations with wildly different
costs into one task: PRODUCING the report (Stryker mutates 2,777 mutants across
client/src — measured 2026-08-27 at 62 minutes in the Linux container this Mac
needs: 4 minutes of setup plus Stryker from 10:16:33 to 11:14:54) and CHECKING
it (check-ts-mutation.py against the stored report — 2.7 seconds, same date). Only the second
is needed when the checker or the adjudication file changes, and only the first
is needed when the client changes. On 2026-08-27 six commits were pushed, ZERO
of which touched anything Stryker mutates, and the 62 minutes were spent anyway.

So: hash the report's inputs, record the hash beside the report, and re-produce
only when they differ. Everything else runs the cheap half every time, which is
what makes it affordable to run the gate at all — and an affordable gate is one
that actually runs, which is the whole problem (ci.yml has been
workflow_dispatch-only since 2026-08-10).

WHAT COUNTS AS AN INPUT, and the answer is deliberately generous: the mutated
sources, the tests that decide whether a mutant lives, the Stryker config, and
the dependency lockfile. A narrower set would be faster to hash and would
eventually miss something — a test helper edited, a dependency bumped — and the
failure mode of missing an input is a stale report reported as current, which is
strictly worse than re-running for nothing. Hashing costs milliseconds.

NOT the checker and NOT the adjudication file. Those change the VERDICT, not the
report, and the verdict is recomputed from the stored report every run.

Exit non-zero and print nothing on any failure: the caller must read that as
"produce the report", never as "the inputs are unchanged".
"""

import hashlib
import json
import os
import re
import sys

# The report's inputs, as directories walked for a suffix or as single files.
# client/src and client/test come from stryker.conf.json's own `mutate` list and
# commandRunner (`bun test client/test`) — read there rather than duplicated
# here would be better, but the glob syntax is Stryker's and parsing it to stay
# honest costs more than it saves; test_the_hashed_trees_match_what_stryker_
# actually_mutates, in ts_mutation_inputs_test.py, pins these against the
# config instead.
TREES = (("client/src", (".ts", ".tsx", ".mts", ".cts")),
         ("client/test", (".ts", ".tsx", ".mts", ".cts")))
FILES = ("stryker.conf.json", "package.json", "bun.lock", "bun.lockb",
         "client/tsconfig.json")


def stryker_version(root):
    """Stryker's pinned version, or "" — a mutator upgrade moves verdicts."""
    for name in ("package.json",):
        path = os.path.join(root, name)
        if not os.path.isfile(path):
            continue
        try:
            with open(path, encoding="utf-8") as fh:
                pkg = json.load(fh)
        except (OSError, ValueError):
            return ""
        for section in ("devDependencies", "dependencies"):
            for dep, ver in (pkg.get(section) or {}).items():
                if re.search(r"stryker", dep, re.I):
                    return f"{dep}@{ver}"
    return ""


def inputs_hash(root="."):
    """SHA256 over the report's inputs, or None if any of it cannot be read."""
    digest = hashlib.sha256()
    try:
        digest.update(stryker_version(root).encode("utf-8"))
        paths = []
        for tree, suffixes in TREES:
            base = os.path.join(root, tree)
            if not os.path.isdir(base):
                return None  # a mutated tree that is not there is not "unchanged"
            for dirpath, _dirs, names in os.walk(base):
                for n in names:
                    if n.endswith(suffixes):
                        paths.append(os.path.join(dirpath, n))
        for f in FILES:
            p = os.path.join(root, f)
            if os.path.isfile(p):
                paths.append(p)
        if not paths:
            return None
        for p in sorted(paths):
            digest.update(os.path.relpath(p, root).encode("utf-8"))
            with open(p, "rb") as fh:
                digest.update(fh.read())
        return digest.hexdigest()
    except OSError:
        return None


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    h = inputs_hash(root)
    if not h:
        print("ts-mutation-inputs: could not hash the report's inputs", file=sys.stderr)
        return 1
    print(h)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
