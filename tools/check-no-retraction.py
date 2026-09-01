#!/usr/bin/env python3
"""Fail when a retraction IDENTIFIER survives anywhere in this tree.

Retraction left the platform on 2026-08-30 (Patrik): a retraction exists to
make something not have happened, and it cannot do that — you cannot unring a
bell. Sub-project 13 took it out of the contract, the fold, the campaign, the
gateway, the harness, the agent and the client. This gate is what stops it
coming back one helper at a time, and it is the spec's exit criterion 1, which
asks for a GATE rather than a search: a grep somebody ran once proves nothing
about tomorrow.

WHY THIS IS NOT A `grep -rin retract`. The word is still all over this tree
and every occurrence is deliberate — dated change-records saying what
retraction used to do at that spot and why the shape changed, which are the
reason the next reader does not re-derive it from first principles. Measured
2026-09-01 over the files this script scans: 268 occurrences across 60 files
in 14 directories (internal 106, tools 78, client 43, contract 38, cmd 3). A
naive grep reds on all 268, and the only way to green it is to exclude those
60 files, at which point the gate enforces nothing. So this gate MASKS
comments and string literals first and looks only at what is left: code
positions. An identifier is what a reintroduction actually looks like, and
there were 14 of those, all of them enforcement (see EXEMPT).

WHAT THIS COVERS, AND WHAT contract/events.test.ts COVERS. They are different
halves and neither substitutes for the other:

  - contract/events.test.ts reads the GENERATED DESCRIPTORS and asserts no
    message name and no oneof arm matches /retract/i. That is the stronger
    check for the wire, because it inspects what consumers compile against —
    a message deleted from the .proto but left in committed generated code
    would still be on the wire, and this script would call that clean (it
    skips generated output on purpose; check:drift owns that).
  - This script reads SOURCE across every language in the tree and catches a
    reintroduced helper — a Go function, a TypeScript export, a proto field
    name (which events.test.ts does not read), a tool script. It says nothing
    about the descriptors.

Both are needed. Delete either and the other passes anyway.

SCOPE. The whole repository from the given root, so a new top-level package
joins the gate by existing — the same reasoning check:invariants uses for
pointing semgrep at a directory. Skipped: generated output (contract/gen,
cmd/vtt/tools.json's siblings), vendored and built trees (node_modules,
webdist, .stryker-tmp), and the untracked scratch dirs.

NOT SCANNED, deliberately: .json. Every occurrence there would be inside a
string, so masking leaves nothing to match; scenario files and cmd/vtt/
tools.json name commands that cannot exist without a contract message, and
events.test.ts refuses those structurally.

KNOWN LIMIT, stated rather than hidden: inside a Python f-string the
interpolated expression is masked along with the literal, so an identifier
used only there is invisible to this gate. TypeScript template literals do
NOT have that hole — `${...}` is unmasked and scanned as the code it is.

WHEN A HIT IS CORRECT CODE, it goes in EXEMPT below with a reason, and it
names the WORDS rather than the file wherever it can: the sites that assert
retraction's absence must be able to write the pattern down, and a whole-file
exclusion would have made the strongest enforcement site the one blind spot.
ONE ENTRY IS WHOLE-FILE and is marked as such — this gate's own boundary
tests, whose fixtures ARE the thing being looked for. Rule 2 forbids weakening
a gate to pass it, so the pressure lands here, in public, with prose attached.

Run: python3 tools/check-no-retraction.py [root]
"""

import os
import re
import sys

NEEDLE = "retract"

# EVERY JS/TS SPELLING, not just `.ts`. The tree has no `.js`, `.jsx`, `.tsx`,
# `.mjs`, `.cjs`, `.mts` or `.cts` file under the source roots today, so this is
# latent — but a gate whose scope is "the extension we happen to use" reports
# clean the day somebody adds one, and closing it costs a line. Caveat stated
# rather than discovered: JSX TEXT between tags is not a string literal to this
# masker, so visible copy containing the word would be reported. No such file
# exists to check that against; reword or EXEMPT it if one arrives.
SOURCE_SUFFIXES = {
    ".go": "c", ".proto": "c", ".py": "py",
    ".ts": "ts", ".tsx": "ts", ".mts": "ts", ".cts": "ts",
    ".js": "ts", ".jsx": "ts", ".mjs": "ts", ".cjs": "ts",
}

# Skipped at ANY depth: vendored trees, and dotted directories (the same rule
# check-mutation.py's fingerprint walk uses) which covers .git, .superpowers,
# .stryker-tmp, .tscov and the e2e artifact dirs.
SKIP_ANY_DEPTH = {"node_modules"}

# Skipped ONLY at these exact paths, relative to the scan root.
#
# THE ANCHORING IS THE POINT, and it closes a hole review found on 2026-09-01:
# skipping a directory called `gen` by NAME skips it at every depth, so
# `internal/foo/gen/hidden.go` — source somebody wrote, in a package the gate
# is supposed to cover — went unscanned, and a `func retractSecret()` there was
# reported clean. Anchoring makes this list rot in the safe direction: a new
# generated tree that nobody adds here gets SCANNED, which is a loud false
# positive rather than a silent hole.
SKIP_PATHS = {
    "reports", "coverage",
    "contract/gen", "cmd/vtt/webdist",
    "contract-spike/proto/gen", "contract-spike/openapi/gen",
    "contract-spike/jsonschema/gen",
}

WORD = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")

# path -> (allowed words or None for the whole file, reason).
#
# Every entry is a site whose JOB is to assert that retraction is gone, so the
# word has to survive there for the absence to be enforced at all. A gate that
# deleted its own enforcement would pass while proving nothing.
EXEMPT = {
    # The constant holding the pattern read against the generated descriptors.
    # A retraction helper hidden elsewhere in this file is still caught.
    "contract/events.test.ts": (
        {"RETRACTION"},
        "the descriptor test's own pattern constant",
    ),
    # `const retractors = Object.keys(commands).filter(...)` — the command
    # surface's proof that no builder can retract.
    "client/test/command-surface.test.ts": (
        {"retractors"},
        "the command-surface test's own result binding",
    ),
    # This gate's boundary tests build synthetic retraction code on purpose;
    # their method names say so. Whole file, because the fixtures ARE the
    # thing being looked for.
    "tools/check_no_retraction_test.py": (None, "this gate's own test corpus"),
}


def _mask_c_like(src, backtick, regex):
    """Blank comments and string literals, keeping every offset and newline.

    `backtick` is "raw" (Go), "template" (TypeScript) or None (proto, where a
    backtick is an ordinary character). `regex` enables JavaScript regular-
    expression literals, whose body is a pattern rather than code.
    """
    out = list(src)
    n = len(src)
    i = 0
    # The last significant character seen, used only to tell a regex literal
    # from a division. A regex may follow an operator or an opener; a division
    # follows a value — an identifier, a number, `)` or `]`.
    prev = ""

    def blank(a, b):
        for k in range(a, min(b, n)):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        c = src[i]
        if c == "/" and i + 1 < n and src[i + 1] == "/":
            j = src.find("\n", i)
            j = n if j < 0 else j
            blank(i, j)
            i = j
            continue
        if c == "/" and i + 1 < n and src[i + 1] == "*":
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
            blank(i, j)
            i = j
            continue
        if regex and c == "/" and (prev == "" or prev in "(,=:[!&|?{};+-*%~^<>"):
            # A regex literal cannot span a newline. If no unescaped `/` closes
            # it on this line it was a division after all, so nothing is
            # blanked and the damage of a wrong guess is bounded to one line.
            j = i + 1
            closed = -1
            in_class = False
            while j < n and src[j] != "\n":
                if src[j] == "\\":
                    j += 2
                    continue
                if src[j] == "[":
                    in_class = True
                elif src[j] == "]":
                    in_class = False
                elif src[j] == "/" and not in_class:
                    closed = j
                    break
                j += 1
            if closed >= 0:
                blank(i, closed + 1)
                i = closed + 1
                prev = ")"  # a regex literal is a value, like any other
                continue
        if c == "`" and backtick == "raw":
            j = src.find("`", i + 1)
            j = n if j < 0 else j + 1
            blank(i, j)
            i = j
            prev = ")"
            continue
        if c == "`" and backtick == "template":
            j = i + 1
            while j < n:
                if src[j] == "\\":
                    j += 2
                    continue
                if src[j] == "`":
                    j += 1
                    break
                if src[j] == "$" and j + 1 < n and src[j + 1] == "{":
                    # Interpolation is CODE. Blank the literal run up to it,
                    # leave the expression alone, and resume after its `}`.
                    blank(i, j)
                    depth = 0
                    k = j + 1
                    while k < n:
                        if src[k] == "{":
                            depth += 1
                        elif src[k] == "}":
                            depth -= 1
                            if depth == 0:
                                break
                        k += 1
                    j = k + 1
                    i = j
                    continue
                j += 1
            blank(i, j)
            i = j
            prev = ")"
            continue
        if c == '"' or c == "'":
            j = i + 1
            while j < n:
                if src[j] == "\\":
                    j += 2
                    continue
                if src[j] == c or src[j] == "\n":
                    j += 1
                    break
                j += 1
            blank(i, j)
            i = j
            prev = ")"
            continue
        i += 1
        if not c.isspace():
            prev = c
    return "".join(out)


def _mask_py(src):
    """Blank `#` comments and every string literal, offsets preserved."""
    out = list(src)
    n = len(src)
    i = 0

    def blank(a, b):
        for k in range(a, min(b, n)):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        c = src[i]
        if c == "#":
            j = src.find("\n", i)
            j = n if j < 0 else j
            blank(i, j)
            i = j
            continue
        if c in "\"'":
            triple = src[i:i + 3]
            if triple in ('"""', "'''"):
                j = i + 3
                while j < n:
                    if src[j] == "\\":
                        j += 2
                        continue
                    if src[j:j + 3] == triple:
                        j += 3
                        break
                    j += 1
                blank(i, j)
                i = j
                continue
            j = i + 1
            while j < n:
                if src[j] == "\\":
                    j += 2
                    continue
                if src[j] == c or src[j] == "\n":
                    j += 1
                    break
                j += 1
            blank(i, j)
            i = j
            continue
        i += 1
    return "".join(out)


def mask(src, kind):
    if kind == "py":
        return _mask_py(src)
    if kind == "ts":
        return _mask_c_like(src, backtick="template", regex=True)
    return _mask_c_like(src, backtick="raw", regex=False)


def source_files(root):
    for dirpath, dirnames, filenames in os.walk(root):
        here = os.path.relpath(dirpath, root).replace(os.sep, "/")
        prefix = "" if here == "." else here + "/"
        dirnames[:] = sorted(
            d for d in dirnames
            if not d.startswith(".")
            and d not in SKIP_ANY_DEPTH
            and prefix + d not in SKIP_PATHS)
        for name in sorted(filenames):
            suffix = os.path.splitext(name)[1]
            kind = SOURCE_SUFFIXES.get(suffix)
            if kind is None:
                continue
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root).replace(os.sep, "/")
            yield full, rel, kind


def findings(root):
    """(rel, line, word) hits, the file count, and anything unreadable.

    A SOURCE FILE THIS GATE CANNOT READ IS ONE IT CANNOT CLEAR. Skipping it
    silently — as this did until review on 2026-09-01 — hid it from the
    scanned count too, so it could not even trip the "scanned no files" guard
    below. It is now reported and fails the gate.
    """
    hits = []
    unreadable = []
    scanned = 0
    for full, rel, kind in source_files(root):
        try:
            src = open(full, encoding="utf-8").read()
        except (OSError, UnicodeDecodeError) as err:
            unreadable.append((rel, err.__class__.__name__))
            continue
        scanned += 1
        if NEEDLE not in src.lower():
            continue
        allowed, _ = EXEMPT.get(rel, (set(), ""))
        if rel in EXEMPT and allowed is None:
            continue
        code = mask(src, kind)
        for m in WORD.finditer(code):
            word = m.group(0)
            if NEEDLE not in word.lower() or word in allowed:
                continue
            hits.append((rel, src.count("\n", 0, m.start()) + 1, word))
    return hits, scanned, unreadable


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    hits, scanned, unreadable = findings(root)
    if unreadable:
        print("check:no-retraction: could not read %d source file(s), so they "
              "were never cleared:" % len(unreadable), file=sys.stderr)
        for rel, why in unreadable:
            print("  %s: %s" % (rel, why), file=sys.stderr)
        return 1
    if scanned == 0:
        print("check:no-retraction: scanned no files under %s — the gate ran "
              "and enforced nothing" % root, file=sys.stderr)
        return 1
    if hits:
        print("check:no-retraction: retraction is not part of this platform "
              "(Patrik, 2026-08-30); found %d identifier(s):" % len(hits),
              file=sys.stderr)
        for rel, line, word in hits:
            print("  %s:%d: %s" % (rel, line, word), file=sys.stderr)
        print("", file=sys.stderr)
        print("  A comment or a test string saying what retraction USED to do "
              "is fine and is not\n"
              "  reported. This is a code position. If it is enforcement "
              "rather than a leftover,\n"
              "  add the word to EXEMPT in tools/check-no-retraction.py with "
              "the reason.", file=sys.stderr)
        return 1
    print("check:no-retraction: clean (%d files scanned)" % scanned)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
