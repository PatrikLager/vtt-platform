#!/usr/bin/env python3
"""Fail when a doc comment describes a DIFFERENT function than the one it sits on.

WHAT THIS CATCHES, and why it is worth a gate. Insert a function between a doc
comment and the function it documents, and Go says nothing: the comment now
belongs to the newcomer and the original is left bare. Nothing about the code
looks wrong, `go vet` is silent, and the next reader is told about a function
they are not looking at.

FOUND THREE TIMES on its first two runs, all months old:
  - internal/harness/soak.go, twice: `deniedLeakLine`'s doc had ended up above
    `settleTrailingDenials`, and `drainToSequence`'s above `drainFreshCatchUp`.
  - internal/gateway/codec.go: ten lines explaining a 2026-08-07 DATA RACE, and
    why the encode seam moved off a package global, floating above a blank line
    where Go attached them to nothing. `go doc EncodeFrame` printed silence.
I had also just done it myself in internal/gateway/presence.go, where
`displayName` was inserted between `broadcast`'s comment and `broadcast` —
caught in review, which is not a mechanism that scales.

TWO SHAPES, because gofmt produces both. Insert a function that brings its own
doc and you get `orphan / newcomer's doc / func` (soak.go) if the blocks touch,
or `orphan / BLANK / newcomer's doc / func` (codec.go) if they do not. The
second is the commoner one and the more dangerous: a blank line means Go
attaches the block to NOTHING, so the prose is not merely misfiled, it is
unreachable from `go doc` entirely.

THE RULE IS DELIBERATELY NARROW: a block is a problem only when its first word
names ANOTHER FUNCTION THAT EXISTS in the tree. That is the fingerprint of a
comment separated from its owner, and it is hard to trip by accident — measured
over this tree, 282 of 284 documented functions already open with their own
name, and the margin is not luck: the two exceptions are the `Complexity:`
blocks below.

It is NOT "every doc must start with its function's name". Go's convention says
that, and staticcheck's ST1020 will enforce it for EXPORTED functions if you
ever want it — but all three defects above were unexported, so ST1020 would
have found none of them. Enforcing it here would flag the deliberate
`// Complexity: ...` blocks above `//nolint:gocyclo` in internal/rules — both
in load.go, and that is the WHOLE cost: 2 sites out of 284. The number is here
so the trade can be re-decided rather than inherited. It is refused today
because a gate whose first act is to demand the rewriting of correct code is a
gate that gets switched off, and because the strict rule buys only the cases
where an orphaned doc opens with prose instead of a name.

WHEN IT IS WRONG, adjudicate rather than reword: put `//doc-owner:ok <reason>`
in the block. Prose that legitimately opens by naming another function ("unlike
`Foo`, this…") is correct code, and rule 2 forbids weakening a gate to pass it,
so the pressure has to land somewhere that carries a reason.

Run: python3 tools/check-doc-owner.py [root]
"""

import pathlib
import re
import sys

FUNC_RE = re.compile(r"^func (?:\([^)]*\) )?([A-Za-z_]\w*)[\[(]")
HATCH = "doc-owner:ok"
SKIP_DIRS = {"gen", "node_modules", ".git", "contract-spike"}


def go_files(root):
    """Every non-test .go file under root, excluding generated and vendored trees."""
    for path in sorted(pathlib.Path(root).rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        if SKIP_DIRS.isdisjoint(path.parts):
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


def is_doc(line):
    """A top-level comment line. Column 0 only: an indented // is inside a body,
    where a comment naming the helper on the next line is ordinary code and a
    blank line after it means nothing at all."""
    return line.startswith("//")


def text(line):
    """A comment line's prose. lstrip FIRST, so that reading the text does not
    quietly depend on is_doc having rejected indentation — with both coupled,
    widening is_doc to accept an indented // slices the text at the wrong
    offset, the word comes out as `/`, and nothing fires. Two decisions that
    only look right because each hides the other's mistake."""
    return line.lstrip().lstrip("/").strip()


def described(block, known):
    """The function a comment block opens by naming, or None.

    The FIRST non-empty line, not block[0]: a block may open with a bare `//`,
    and reading that literally yields an empty word that skips the block whole.
    """
    if any(b.startswith(HATCH) for b in block):
        return None
    first = next((b for b in block if b), "")
    if not first:
        return None
    word = first.split()[0].strip("`*,.:'\"")
    return word if word in known else None


def doc_block(lines, i):
    """The contiguous comment block immediately above lines[i]."""
    block, j = [], i - 1
    while j >= 0 and is_doc(lines[j]):
        block.insert(0, text(lines[j]))
        j -= 1
    return block


def floating_blocks(lines):
    """Every comment block that Go attaches to NOTHING — one followed by a blank
    line, or by end of file. Yields (line_number, block)."""
    i = 0
    while i < len(lines):
        if not is_doc(lines[i]):
            i += 1
            continue
        start = i
        while i < len(lines) and is_doc(lines[i]):
            i += 1
        if i >= len(lines) or lines[i].strip() == "":
            yield start + 1, [text(ln) for ln in lines[start:i]]


def scan(root="."):
    """(files scanned, findings). A finding is (path, line, owner|None, named)."""
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
            named = described(doc_block(lines, i), known)
            if named and named != name:
                out.append((str(path), i + 1, name, named))
        for line_no, block in floating_blocks(lines):
            named = described(block, known)
            if named:
                out.append((str(path), line_no, None, named))
    return paths, out


def report(path, line, owner, named):
    if owner is None:
        return (
            f"check:doc-owner: {path}:{line}: this comment block documents NOTHING. It "
            f"begins by describing `{named}`, but a blank line separates it from what "
            f"follows, and Go attaches a doc comment only to a declaration that comes "
            f"directly after it — so `go doc {named}` prints none of this. Either close "
            f"the gap so it lands on `{named}`, or move it to wherever `{named}` now lives."
        )
    return (
        f"check:doc-owner: {path}:{line}: the doc comment above `{owner}` begins by "
        f"describing `{named}`, which is a different function. Two ways that happens, and "
        f"the gate cannot tell them apart: a function was inserted between that comment "
        f"and the one it documents — move the comment back to `{named}`, and check whether "
        f"`{owner}` and `{named}` each still have a doc of their own — or this doc really "
        f"is about `{owner}` and merely happens to open by naming `{named}`, in which case "
        f"adjudicate it with a `//{HATCH} <reason>` line rather than rewording it."
    )


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    paths, bad = scan(root)
    if not paths:
        print(
            f"check:doc-owner: no Go files under {root} — nothing was scanned, so nothing "
            f"is proven. A clean run over an empty tree is not a pass.",
            file=sys.stderr,
        )
        return 2
    for finding in bad:
        print(report(*finding), file=sys.stderr)
    if bad:
        return 1
    print(f"check:doc-owner: {len(paths)} files, every doc comment sits on its own function.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
