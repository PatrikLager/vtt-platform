#!/usr/bin/env python3
"""Fail when a doc comment names a DIFFERENT function than the one it sits on.

WHAT THIS CATCHES, and why it is worth a gate. Insert a function between a doc
comment and the function it documents, and Go says nothing: the comment now
belongs to the newcomer and the original is left bare. Nothing about the code
looks wrong, `go vet` is silent, and the next reader is told about a function
they are not looking at.

FOUND TWICE ON ITS FIRST RUN, both in internal/harness/soak.go, both months
old: `deniedLeakLine`'s doc had ended up above `settleTrailingDenials`, and
`drainToSequence`'s above `drainFreshCatchUp`. Both original functions had been
left with no documentation at all. I had also just done it myself in
internal/gateway/presence.go, where `displayName` was inserted between
`broadcast`'s comment and `broadcast` — caught in review, which is not a
mechanism that scales.

THE RULE IS DELIBERATELY NARROW: a doc block is a problem only when its first
word names ANOTHER FUNCTION THAT EXISTS in the tree. That is the fingerprint of
a comment separated from its owner, and it is almost impossible to trip by
accident.

It is NOT "every doc must start with its function's name". Go's convention says
that, but enforcing it here would flag the deliberate `// Complexity: ...`
blocks that sit above `//nolint:gocyclo` in internal/rules — a real local
convention with a reason — and a gate whose first act is to demand the removal
of correct code is a gate that gets switched off. Catch the defect, not the
style.

Run: python3 tools/check-doc-owner.py [root]
"""

import pathlib
import re
import sys

FUNC_RE = re.compile(r"^func (?:\([^)]*\) )?([A-Za-z_]\w*)\(")


def go_files(root):
    """Every non-test .go file under root, excluding generated and vendored trees."""
    skip = ("/gen/", "/node_modules/", "/.git/", "contract-spike/")
    for path in sorted(pathlib.Path(root).rglob("*.go")):
        p = str(path)
        if path.name.endswith("_test.go") or any(s in p for s in skip):
            continue
        yield path


def declared_functions(paths):
    """Every function name declared across paths."""
    names = set()
    for path in paths:
        for line in path.read_text(encoding="utf-8").split("\n"):
            m = FUNC_RE.match(line)
            if m:
                names.add(m.group(1))
    return names


def doc_block(lines, i):
    """The contiguous // comment block immediately above lines[i]."""
    block, j = [], i - 1
    while j >= 0 and lines[j].lstrip().startswith("//"):
        block.insert(0, lines[j].lstrip()[2:].strip())
        j -= 1
    return block


def findings(root="."):
    paths = list(go_files(root))
    known = declared_functions(paths)
    out = []
    for path in paths:
        lines = path.read_text(encoding="utf-8").split("\n")
        for i, line in enumerate(lines):
            m = FUNC_RE.match(line)
            if not m:
                continue
            name = m.group(1)
            block = doc_block(lines, i)
            first = next((b for b in block if b), "")
            if not first:
                continue
            word = first.split()[0].strip("`*,.:")
            if word != name and word in known:
                out.append((str(path), i + 1, name, word))
    return out


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    bad = findings(root)
    for path, line, owner, named in bad:
        print(
            f"check:doc-owner: {path}:{line}: the doc comment above `{owner}` begins by "
            f"describing `{named}`, which is a different function. A function was almost "
            f"certainly inserted between that comment and the one it documents — move the "
            f"comment back to `{named}` rather than rewriting it, and check whether "
            f"`{owner}` and `{named}` each still have a doc of their own.",
            file=sys.stderr,
        )
    if bad:
        return 1
    print(f"check:doc-owner: {len(list(go_files(root)))} files, every doc comment sits on its own function.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
