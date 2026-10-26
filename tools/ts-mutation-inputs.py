#!/usr/bin/env python3
"""Print a hash of everything that can change the TS mutation REPORT.

WHAT THE HASH DECIDES. The full run, `task check:ts-mutation:full`, stamps the
report it produces with this hash (tools/ts-mutation-report.py). `task check`
judges the stored report only while the tree hashes to the stamp's value; any
other value means the report describes a different client, and the step says
the client's mutation was not measured instead (SPEC-023). It never starts
Stryker, which runs when the owner asks for it.

WHAT COUNTS AS AN INPUT, and the answer is deliberately generous: the mutated
sources, the tests that decide whether a mutant lives, every file outside
client/ that a client source or test imports or reads by a path written in it,
the Stryker config, and the dependency lockfile. A narrower set would be faster
to hash and would eventually miss something — a test helper edited, a golden
regenerated, a dependency bumped — and the failure mode of missing an input is a
stale report reported as current, which is strictly worse than a warning that
was not needed. Hashing costs milliseconds.

NOT the checker and NOT the adjudication file. Those change the VERDICT, not the
report, and the verdict is recomputed from a current report every run.

Exit non-zero and print nothing on any failure: the caller must read that as
"out of date", never as "the inputs are unchanged".
"""

import hashlib
import json
import os
import re
import sys

# The report's inputs, as directories walked for a suffix or as single files.
# client/src and client/test come from stryker.conf.json's own `mutate` list
# and commandRunner (`bun test --bail --only-failures client/test`) — read
# there rather than duplicated here would be better, but the glob syntax is
# Stryker's and parsing it to stay honest costs more than it saves;
# test_the_hashed_trees_match_what_stryker_actually_mutates, in
# ts_mutation_inputs_test.py, pins these against the config instead.
#
# The rest are what the client reaches outside client/ (VTT-516): the generated
# contract it imports; contract/testdata, which tests read by a name passed in;
# the goldens fold-parity.test.ts compares the fold against; the tool
# definitions two QA tests read; and the example campaign's art, which
# art-assets.test.ts serves, and its map, which it reads. The art's suffixes
# are internal/artlib's, so a stray .DS_Store moves nothing.
# test_every_path_the_client_writes_outside_client_is_hashed holds the next
# path written in the client; a path built from pieces it cannot see.
TREES = (("client/src", (".ts", ".tsx", ".mts", ".cts")),
         ("client/test", (".ts", ".tsx", ".mts", ".cts")),
         ("contract/gen/ts", (".ts",)),
         ("contract/testdata", (".json",)),
         ("scenarios/goldens", (".json",)),
         ("campaigns/example/art", (".png", ".json")))
FILES = ("stryker.conf.json", "package.json", "bun.lock", "bun.lockb",
         "client/tsconfig.json", "contract/gen/tools/tools.json",
         "campaigns/example/maps/cellar.json")


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
        if not isinstance(pkg, dict):
            return ""
        for section in ("devDependencies", "dependencies"):
            deps = pkg.get(section)
            if not isinstance(deps, dict):
                continue
            for dep, ver in deps.items():
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
