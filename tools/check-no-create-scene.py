#!/usr/bin/env python3
"""Fail when a scene-MAKING COMMAND identifier survives anywhere in this tree.

create_scene left the platform on 2026-09-01 (Patrik): the kernel serves maps,
it does not make them. Building a place square by square is iterative work, and
a one-shot command that emits a whole scene can never edit what it emitted —
the DM console's wall fill proved it by making a room no player could ever
enter, permanently, with the only repair (retraction) already deleted. A future
map editor's output arrives the way any other map does: as a file, through
load_map. Sub-project 15 took the command out of the contract, the gateway, the
client, the agent tool surface, the golden corpus and the soak. This gate is
what stops it coming back one helper at a time.

WHAT IS REMOVED IS THE COMMAND, NOT THE EVENT, and that distinction is this
gate's whole shape. `SceneCreated` is alive: mapdef.Compile emits it, the
adventure loader emits it, engine.Apply folds it, gateway/project.go builds a
redacted per-viewer copy of it, and it is on the wire. A needle matching
"scene" — or "create" anywhere near "scene" — would red on the healthy half of
the platform and be switched off inside a day. So the needle is the joined
identifier `createscene` and nothing else. `SceneCreated` normalises to
`scenecreated`, which does not contain it. See TheEventIsNotTheCommand in the
self-test, which is the boundary most likely to be eroded by a later widening.

WHY THIS IS NOT A `grep -rin create_scene`. The words are still all over this
tree and every occurrence is deliberate — dated change-records saying what
create_scene used to do at that spot and why the shape changed, which are the
reason the next reader does not re-derive it from first principles. Measured
2026-09-02 over the tree as this gate found it, with the same filter the plan
used (node_modules, contract/gen, webdist, .stryker-tmp and the dotted scratch
excluded): 111 occurrences across 42 files, 35 of which this gate scans — and
in every one of those the word is a comment, a string literal or a
plan-directory name inside a citation. Under the masker below: ZERO code
positions. A naive grep reds on all 111, and the only way to green it is to
exclude those 42 files, at which point the gate enforces nothing. So this gate
MASKS comments and string literals first and looks only at what is left.

THAT COUNT IS A DATED OBSERVATION, NOT THE CURRENT SHAPE, and the distinction
is the point: the raw number went UP the moment this gate arrived, because
this file, its self-test and its Taskfile entry all have to write the
identifier down in order to enforce its absence. Do not re-pin it on every
edit — it rots by addition, which is the failure this repo keeps writing down.
The invariant is the sentence that does not rot: ZERO code positions, over
every file this gate scans, which is what a green run asserts.

THREE SPELLINGS, ONE IDENTIFIER. The contract writes `create_scene`, the
TypeScript client `createScene`, Go `CreateScene`, and generated Go joins the
two as `ClientCommand_CreateScene`. A word is normalised by lower-casing it and
DELETING underscores, which folds all four onto one needle. Underscores are
deleted rather than treated as separators precisely so the generated wrapper
name matches when a hand-written switch arm spells it out.

WHAT THIS COVERS, AND WHAT client/test/command-surface.test.ts COVERS. They are
different halves and neither substitutes for the other:

  - command-surface.test.ts reads the GENERATED TS DESCRIPTOR. commandCases()
    walks ClientCommandSchema's `command` oneof, one test asserts no builder
    key matches /createScene/i, and two more assert the oneof and the surface
    table name the same set. That is the stronger check for the wire, because
    it inspects what consumers compile against — an arm deleted from the .proto
    but left in committed generated code would still be on the wire, and this
    script would call that clean (it skips generated output on purpose;
    check:drift owns that).
  - This script reads SOURCE in the twelve extensions listed in
    SOURCE_SUFFIXES — .go, .proto, .py, the eight JS/TS spellings and .json —
    and catches a reintroduced helper there: a Go function, a TypeScript
    export, a proto field name, a Python tool, a scenario or golden JSON key,
    none of which the descriptor test can see. It says nothing about the
    descriptors, and nothing about any other file type (see KNOWN LIMITS).

Both are needed. Delete either and the other passes anyway. Note the third leg
the retraction gate had is ABSENT here: contract/events.test.ts sweeps the
descriptors for /retract/i and has no create_scene equivalent, so do not read
this paragraph as claiming one.

SCOPE IS TWO FILTERS, AND ONLY ONE OF THEM IS DIRECTORIES. Read both before
concluding a file is covered.

By DIRECTORY: the whole repository from the given root, so a new top-level
package joins the gate by existing — the same reasoning check:invariants uses
for pointing semgrep at a directory. Skipped: generated output (contract/gen,
the contract-spike gen trees), built trees (cmd/vtt/webdist), vendored
node_modules, the report/coverage output dirs, and every dotted directory
(.git, .superpowers, .stryker-tmp, .tscov, the e2e artifact dirs).

By EXTENSION: only the twelve in SOURCE_SUFFIXES. Everything else in those
directories is invisible to this gate, which is the largest limit it has and
is stated first among them below.

.json IS SCANNED, and only its OBJECT KEYS are. Inherited whole from
check-no-retraction.py, which paid a review round to learn it: in JSON the key
is the code position — it is the protobuf field name a scenario step or a
golden stream is naming — while every VALUE is data, so `_mask_json` keeps key
text and blanks the rest. A scenario NAME or a narration saying what
create_scene used to do is therefore safe, and a `create_scene` step anywhere
in the corpus is not.

KNOWN LIMITS, stated rather than hidden. This list is meant to be
exhaustive; if you find a hole that is not here, the omission is the defect:

  - FILE TYPES OUTSIDE SOURCE_SUFFIXES ARE NOT READ AT ALL, and this is the
    biggest gap. Measured inside this gate's own directory scope on
    2026-09-02: 5 .yaml, 4 .yml (including .go-arch-lint.yml and Taskfile.yml),
    2 .sh under tools/, and client/index.html. Verified by injection, not by
    reading the code: `createScene: true` appended to .go-arch-lint.yml,
    `create_scene() { :; }` appended to tools/list-measured-packages.sh, and
    `<script>function createScene(){}</script>` appended to client/index.html
    all leave this gate reporting `clean`. An inline <script> in index.html is
    the sharpest of the three, because it is real executable client code. The
    .md files are excluded on purpose — prose is what this gate exists to
    preserve — but YAML, shell and HTML are not a decision, they are a gap.
  - A JSON string VALUE naming the command is invisible. The one place that
    matters is a tool manifest, where identity is spelled `"name": "load_map"`
    — a value. Two DIFFERENT controls cover the two manifests, and neither is
    this gate: cmd/vtt/tools.json is rewritten from toolgen's output by
    `task generate:contract` and then diffed by check:drift; contract/testdata/
    expected_tools.json is neither generated nor in check:drift's pathspec, and
    is held instead by TestToolsMatchGolden in tools/toolgen/main_test.go,
    which compares it against buildTools() reading the compiled descriptors.
    Either way a create_scene tool needs a `create_scene` arm in commands.proto,
    and that arm IS a code position this gate sees.
  - A FILE NAME is not scanned, only file contents. internal/gateway/
    create_scene_validate.go and its test were deleted by this sub-project and
    could return under the same names; in practice such a file's contents carry
    the identifier too, and that is what is checked. This gate's own two files
    are named for the identifier, which is why scanning names was never an
    option.
  - Inside a Python f-string the interpolated expression is masked along with
    the literal, so an identifier used only there is invisible. TypeScript
    template literals do NOT have that hole — `${...}` is unmasked and scanned
    as the code it is.
  - A REGEX LITERAL AFTER A KEYWORD IS A FALSE POSITIVE. The `/`-vs-division
    heuristic starts a regex only after one of ``(,=:[!&|?{};+-*%~^<>`` or at
    input start, so `return /createScene/i.test(k);` is read as division and
    the identifier inside the pattern is REPORTED. Verified by injecting
    exactly that line into client/src/commands.ts, which reds the gate.
    command-surface.test.ts's real assertion sits after `=>` and is safe — but
    rewording it to a `return` would red this gate and create pressure for the
    first EXEMPT entry, which is the pressure the word-level design exists to
    resist. Widen the prev-character set rather than exempting the file.
  - A RENAME defeats any pattern. `buildWholeScene` is not this needle, and no
    gate here can say otherwise; what this one guarantees is that the shape
    cannot come back wearing its own name.

THE MASKER BELOW IS A DELIBERATE COPY of check-no-retraction.py's, not a shared
import. Two gates that differ only in their needle could have been one module,
and the plan for this task named two new files and Taskfile.yml — refactoring a
gate that is already green is its own reviewed decision under CLAUDE.md rule 2,
not something to fold into the task that has to run the whole suite from cold.
THE OBLIGATION THAT CREATES: a masking bug found in either file is a bug in
both, and the fix has to be applied twice. Each copy is pinned by its own
boundary tests, so a divergence introduced on purpose is visible; one
introduced by fixing only one side is not.

WHEN A HIT IS CORRECT CODE, it goes in EXEMPT below with a reason, and it names
the WORDS rather than the file: the sites that assert this command's absence
must be able to write the pattern down, and a whole-file exclusion would make
the strongest enforcement site the one blind spot. EXEMPT IS EMPTY TODAY and
the self-test pins that it is empty, because nothing in this tree needs to
write the identifier in a code position — the client's absence assertion is a
regex literal, whose body is a pattern rather than code. Rule 2 forbids
weakening a gate to pass it, so the pressure of the first entry lands here, in
public, with prose attached.

Run: python3 tools/check-no-create-scene.py [root]
"""

import os
import re
import sys

# The joined identifier, after lower-casing and deleting underscores. NOT
# "scene" and NOT "create": see the module docstring — the event stays.
NEEDLE = "createscene"

# EVERY JS/TS SPELLING, not just `.ts`. A gate whose scope is "the extension
# we happen to use today" reports clean the day somebody adds one, and closing
# it costs a line. CAVEAT CARRIED OVER FROM check-no-retraction.py, because it
# still applies and a future JSX author would otherwise never read it: JSX TEXT
# between tags is not a string literal to this masker, so visible copy
# containing the identifier would be REPORTED. No .jsx or .tsx file exists in
# the source roots today to check that against — but both are scanned, and
# test_every_javascript_and_typescript_spelling_is_scanned pins that they are.
# Reword or EXEMPT such a file if one arrives.
SOURCE_SUFFIXES = {
    ".go": "c", ".proto": "c", ".py": "py",
    ".ts": "ts", ".tsx": "ts", ".mts": "ts", ".cts": "ts",
    ".js": "ts", ".jsx": "ts", ".mjs": "ts", ".cjs": "ts",
    ".json": "json",
}

# Skipped at ANY depth: vendored trees. Dotted directories are skipped by the
# walk below (the same rule check-mutation.py's fingerprint walk uses), which
# covers .git, .superpowers, .stryker-tmp, .tscov and the e2e artifact dirs.
SKIP_ANY_DEPTH = {"node_modules"}

# Skipped ONLY at these exact paths, relative to the scan root.
#
# THE ANCHORING IS THE POINT. Skipping a directory called `gen` by NAME skips
# it at every depth, so `internal/foo/gen/hidden.go` — source somebody wrote,
# in a package the gate is supposed to cover — would go unscanned. Anchoring
# makes this list rot in the safe direction: a new generated tree that nobody
# adds here gets SCANNED, which is a loud false positive rather than a silent
# hole.
SKIP_PATHS = {
    "reports", "coverage",
    "contract/gen", "cmd/vtt/webdist",
    "contract-spike/proto/gen", "contract-spike/openapi/gen",
    "contract-spike/jsonschema/gen",
}

WORD = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")

# path -> (allowed words, or None for the whole file, reason).
#
# EMPTY, AND THE SELF-TEST ASSERTS IT IS. Measured 2026-09-02: the tree has no
# code position carrying this identifier. Adding the first entry is a decision
# with prose attached, not a quiet line in a dict.
EXEMPT = {}


def normalize(word):
    """Fold a token onto the needle's alphabet: lower-case, no underscores."""
    return word.replace("_", "").lower()


def _mask_c_like(src, backtick, regex):
    """Blank comments and string literals, keeping every offset and newline.

    `backtick` is "raw" (Go) or None; proto is passed "raw" too, harmlessly,
    since a backtick has no meaning there and none appears in this tree.
    "template" is TypeScript, whose `${...}` interpolations are CODE and are
    left unmasked. `regex` enables JavaScript regular-expression literals,
    whose body is a pattern rather than code.
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


def _mask_json(src):
    """Keep OBJECT KEY text, blank everything else, offsets preserved.

    A key is a string literal whose next non-whitespace character is `:`. That
    is the whole rule, and it is deliberately syntactic rather than a parse:
    an unparseable or half-written fixture still gets scanned, where json.loads
    would raise and the file would go unchecked — the same failure mode the
    unreadable-file guard in findings() exists to refuse.

    Escapes inside a key are left as their raw characters. WORD only ever
    matches identifier runs, so a backslash-u escape contributes the word after
    the backslash rather than the letter it stands for, and a key spelled that
    way would slip past. Nothing in this tree spells a field name in escapes,
    and a fixture that did would be doing it on purpose.
    """
    out = [" " if c != "\n" else "\n" for c in src]
    n = len(src)
    i = 0
    while i < n:
        if src[i] != '"':
            i += 1
            continue
        j = i + 1
        while j < n:
            if src[j] == "\\":
                j += 2
                continue
            if src[j] == '"':
                break
            j += 1
        if j >= n:
            break
        k = j + 1
        while k < n and src[k].isspace():
            k += 1
        if k < n and src[k] == ":":
            for m in range(i + 1, j):
                if src[m] != "\n":
                    out[m] = src[m]
        i = j + 1
    return "".join(out)


def mask(src, kind):
    if kind == "py":
        return _mask_py(src)
    if kind == "json":
        return _mask_json(src)
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
            kind = SOURCE_SUFFIXES.get(os.path.splitext(name)[1])
            if kind is None:
                continue
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root).replace(os.sep, "/")
            yield full, rel, kind


def findings(root, exempt=None):
    """(rel, line, word) hits, the file count, and anything unreadable.

    `exempt` defaults to the shipped EXEMPT table; it is a parameter so the
    self-test can drive the per-word mechanism over a synthetic tree while the
    shipped table stays empty.

    A SOURCE FILE THIS GATE CANNOT READ IS ONE IT CANNOT CLEAR. Skipping it
    silently would hide it from the scanned count too, so it could not even
    trip the "scanned no files" guard in main(). It is reported and fails.
    """
    exempt = EXEMPT if exempt is None else exempt
    hits = []
    unreadable = []
    scanned = 0
    for full, rel, kind in source_files(root):
        try:
            # A `with`, not check-no-retraction.py's bare `open(...).read()`:
            # that idiom leaks the handle until GC, and CPython reports it as
            # a ResourceWarning on stderr under unittest's warning capture —
            # noise in a gate's output, which is the thing readers learn to
            # ignore and then miss a real line inside.
            with open(full, encoding="utf-8") as fh:
                src = fh.read()
        except (OSError, UnicodeDecodeError) as err:
            unreadable.append((rel, err.__class__.__name__))
            continue
        scanned += 1
        # Cheap pre-filter. Normalising the WHOLE file cannot lose a hit that
        # per-word normalising would find: both transforms are per-character,
        # so any word that folds onto the needle folds onto it in place.
        if NEEDLE not in normalize(src):
            continue
        allowed, _ = exempt.get(rel, (set(), ""))
        if rel in exempt and allowed is None:
            continue
        code = mask(src, kind)
        for m in WORD.finditer(code):
            word = m.group(0)
            if NEEDLE not in normalize(word) or word in allowed:
                continue
            hits.append((rel, src.count("\n", 0, m.start()) + 1, word))
    return hits, scanned, unreadable


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    hits, scanned, unreadable = findings(root)
    if unreadable:
        print("check:no-create-scene: could not read %d source file(s), so they "
              "were never cleared:" % len(unreadable), file=sys.stderr)
        for rel, why in unreadable:
            print("  %s: %s" % (rel, why), file=sys.stderr)
        return 1
    if scanned == 0:
        print("check:no-create-scene: scanned no files under %s — the gate ran "
              "and enforced nothing" % root, file=sys.stderr)
        return 1
    if hits:
        print("check:no-create-scene: the kernel serves maps, it does not make "
              "them (Patrik, 2026-09-01); found %d identifier(s):" % len(hits),
              file=sys.stderr)
        for rel, line, word in hits:
            print("  %s:%d: %s" % (rel, line, word), file=sys.stderr)
        print("", file=sys.stderr)
        print("  A comment or a test string saying what create_scene USED to "
              "do is fine and is not\n"
              "  reported, and so is the EVENT SceneCreated, which is alive — "
              "load_map and\n"
              "  load_adventure both emit it. This is a code position naming "
              "the removed COMMAND.\n"
              "  If it is enforcement rather than a leftover, add the word to "
              "EXEMPT in\n"
              "  tools/check-no-create-scene.py with the reason.",
              file=sys.stderr)
        return 1
    print("check:no-create-scene: clean (%d files scanned)" % scanned)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
