#!/usr/bin/env python3
"""Fail when the removed ART CONTAINER survives at a code position in this tree.

The pack left the platform on 2026-09-02 (Patrik): *"each 'art' is unique and
should be able to be used by any map"*, and on subfolders, *"we do not allow
underlying folders. You want a different masonry_1, then you have to call it
something else. very simple."* A pack was an access boundary — `mapdef.Map.Pack`
was one string, every `overrides` value and every `objects[].art` resolved
against it, so a map could use art from exactly one container and art was
reachable only by a map that had claimed the container holding it. The path of
least resistance under that rule is a bespoke pack per map, which is not a
distribution unit. Sub-project 16 replaced it with one flat `art/` directory
where the filename is the identity. This gate is what stops it coming back one
helper at a time, and it is the design spec's exit criterion 7, which asks for a
GATE in this exact shape rather than a search: a grep somebody ran once proves
nothing about tomorrow.

WHY THE BARE WORD CANNOT BE THE NEEDLE, and this gate differs from its two
siblings on exactly this point. `retract` and `create_scene` were coined
identifiers that appeared nowhere else. `pack` is ordinary English AND it is
Go's own keyword. Measured 2026-09-06 over the twelve extensions this gate
reads, over the tree AS THIS GATE FOUND IT: 1857 occurrences of the word across
257 files, and 923 of them are the `package` family (820 `package`, 103
`packages`) — every .go file in the repository opens with one. Over every text
file inside the same directory scope it is 2668 across 319 files, 1310 of them
`package`. A gate matching the bare word reds on line 1 of every Go file, and
the only way to green it is to exempt the whole tree, at which point it enforces
nothing. So the needle is a WORD SHAPE with the English word carved out
structurally, and comments and string literals are masked before anything is
read at all.

THE CARVE-OUT IS `pack` FOLLOWED BY `age` IN THE SAME CASE RUN, and the case is
load-bearing. `package`, `Package`, `PACKAGE`, `packages`, `packaged`,
`packager`, `packageManager`, `go_package`, `PACKAGE_CLAUSE_RE` and
`Subpackage` are all the English word and none is reported. `packAgent`,
`PackAge` and `packAgency` are camel-case boundaries — `pack` followed by `Age`
— and ARE reported, because a container helper called `packAgent` is the
container. Lower-case the word before testing and that distinction is gone,
which is why the needle keeps the case. See the self-test's
TheEnglishWordIsNotTheNeedle, which pins both directions.

THAT CARVE-OUT IS A SUBSTRING CARVE-OUT, AND IT COSTS A WHOLE SPELLING. Read the
list above as reassurance and you will miss what it implies: the rule that
clears `Subpackage` and `packagePath` clears `ArtPackage`, `LoadPackage`,
`handlePackageFile`, `PackageTile`, `packageFS`, `ErrPackageNotLoaded`,
`packageFileURL` and `artPackages` as well, and a map file whose object key is
`"package"` rather than `"pack"`. Verified by injection on 2026-09-06: a Go file
declaring all four of `type ArtPackage`, `func LoadPackage`,
`handlePackageFile` and `ErrPackageNotLoaded`, beside a map carrying
`{"package": "cellar-basics"}`, leaves this gate reporting `clean` and exiting
0. A pack was a DISTRIBUTION UNIT, so "art package" is the most idiomatic rename
anyone would reach for, and it reads as ordinary English to a reviewer. This is
the largest hole in the needle and it is listed again under KNOWN LIMITS.

A TIGHTER NEEDLE IS NOT AVAILABLE, and that was measured rather than assumed:
308 code positions across 39 distinct `package`-family words exist in this tree
(2026-09-06). Any rule permissive enough to clear those clears `ArtPackage` too.
So this is a limit to write down, not a needle to fix — and the two halves that
CAN be closed have been, both in the package that owned the container:
`internal/mapdef`'s `mapJSON.Package` refuses the `"package"` key with the same
migration message `"pack"` gets, and
`TestNoPackTypeOrLoaderRemainsInThisPackage` bans `Package{`, `ArtPackage`,
`LoadPackage` and `PackageTile` alongside the `Pack` forms. Neither closes the
general case, and nothing in this repository does.

WHAT IS CARVED OUT IS ONLY `package`. `packet`, `packed` and `unpack` are NOT,
and that is a decision rather than an oversight: each is one keystroke from the
container's own vocabulary — `unpackArt` is precisely the shape a loader comes
back in — and none of the three occurs at a code position in this tree today
(measured 2026-09-06). `packaging` is likewise not carved out; its single
occurrence in the tree is in a `.md`, which this gate does not read. If one
arrives in a scanned file it goes in EXEMPT with a reason, in public.

WHAT SURVIVES ON PURPOSE, AND WHY EXEMPT IS NOT EMPTY HERE. Two families, and
neither is a leftover:

  - THE STANDARD BASELINE PACK, which keeps its name.
    client/public/std-pack/pack.json is a real manifest generated by
    tools/genmappack and read by client/src/view/pack-assets.ts, whose header
    says so in as many words: "IT IS A PACK AND IT KEEPS THE NAME." It is
    first-party, generated at this repository's own commit, shipped in the same
    build artifact as index.html, and served unauthenticated from `/std-pack/`
    through the gateway's static route — NOT through GET /api/art/{file}, which
    exists for operator-installed content, and not through anything a map's
    claim resolves against. It is a picture for each of internal/mapdef's
    eleven standard natures so a square with no override draws something. What
    LEFT is the per-container half: packFileURL, packManifestURL,
    imageRequestsForPack, loadPackImages and GET /api/packs/{pack}/{file}.
  - THE SITES WHOSE JOB IS TO REFUSE THE CONTAINER. `mapJSON.Pack` is the only
    way loadAs can refuse a file that declares one, and the tests named for
    those refusals have to write the word down to assert its absence.
    internal/mapdef/load_test.go recorded the trap before this gate existed:
    "mapJSON.Pack survives on purpose ... so a bare `Pack` search would fail
    forever and be deleted by whoever hit it."

Both families are EXEMPT BY WORD, never by file, so the module with the most
legitimate mentions is still scanned for every other spelling of the container
— see test_an_exemption_allows_only_the_words_it_names. Measured 2026-09-06:
87 code positions across 13 files, 23 distinct words, every one of them in one
of those two families, and the table names exactly those 13 files and those 23
words with nothing dead in it. The 1857 above is a DATED OBSERVATION and not
the current shape: the raw count rose the moment this gate arrived, because
this file, its self-test and its Taskfile entry all write the word down in
order to enforce its absence. The invariant that does not rot is the sentence a
green run asserts — no code position outside EXEMPT, over every file this gate
scans.

WHAT THIS COVERS, AND WHAT THE TWO GO TESTS COVER. Three different halves, and
none substitutes for another:

  - internal/mapdef's TestNoPackTypeOrLoaderRemainsInThisPackage reads that ONE
    package's non-test .go files for six exact strings — LoadPack, PackTile,
    PackFormatVersion, packJSON, packTileJSON, packTileMap — after a crude
    comment strip. It is the strongest check for a resurrection under the old
    names in the package that owned them, and it is blind to a rename, to test
    files, and to every other package. This gate is blind to none of those and
    knows none of those names: see test_a_rename_of_the_container_is_still_caught.
  - internal/gateway's TestNoPackRouteIsServed asks a RUNNING server for
    /api/packs/{pack}/{file} without a token and demands 404 rather than 401.
    ITS WHOLE SIGNAL IS THAT 401, and the 401 existed only because the deleted
    handlePackFile called s.authed before it looked at anything. A route
    brought back WITHOUT an auth gate answers 404 for an unknown id exactly as
    an absent route does, and that test passes while serving bytes to anyone.
    Its own doc comment says so and names this file as the instrument for it.
  - This script reads SOURCE in the twelve extensions in SOURCE_SUFFIXES and
    catches the code: a Go function, a struct field, a TypeScript export, a
    proto field name, a Python tool, a JSON object key. It is the only one of
    the three that covers a rename, and the only one that covers the whole
    tree. IT IS ALSO THE ONE THAT CANNOT SEE A ROUTE PATTERN: `mux.HandleFunc(
    "/api/packs/{pack}/{file}", s.serveLegacy)` is a string literal, masked
    before anything is read, and a handler named without the word is clean
    here. That is precisely the hole TestNoPackRouteIsServed half-covers, and
    it is why the two are kept rather than merged.

SCOPE IS TWO FILTERS, AND ONLY ONE OF THEM IS DIRECTORIES.

By DIRECTORY: the whole repository from the given root, so a new top-level
package joins the gate by existing — the same reasoning check:invariants uses
for pointing semgrep at a directory. Skipped: generated output (contract/gen,
the contract-spike gen trees), built trees (cmd/vtt/webdist), vendored
node_modules, the report/coverage output dirs, and every dotted directory
(.git, .superpowers, .stryker-tmp, .tscov, the e2e artifact dirs).

By EXTENSION: only the twelve in SOURCE_SUFFIXES. Everything else in those
directories is invisible to this gate, which is the largest limit it has and is
stated first among them below.

.json IS SCANNED, and only its OBJECT KEYS are. Inherited whole from
check-no-retraction.py, which paid a review round to learn it: in JSON the key
is the code position — it is the field name a map file, a scenario step or a
golden stream is naming — while every VALUE is data, so `_mask_json` keeps key
text and blanks the rest. A narration or a scenario name saying what a pack used
to do is therefore safe; `"pack": "cellar-basics"` in a map file is not, and
that is the exact shape design spec §7 refuses with no compatibility layer.
Note the standard baseline's own manifest is clean under this rule without an
exemption: pack.json's keys are id/name/cell_px/tiles/objects, and the file is
merely NAMED for the thing.

KNOWN LIMITS, stated rather than hidden. This list is meant to be exhaustive;
if you find a hole that is not here, the omission is the defect:

  - FILE TYPES OUTSIDE SOURCE_SUFFIXES ARE NOT READ AT ALL, and this is the
    biggest gap. Inside this gate's own directory scope on 2026-09-06 that is
    the .yaml/.yml files (.go-arch-lint.yml and Taskfile.yml among them), the
    .sh scripts under tools/, and client/index.html — whose inline <script> is
    real executable client code. Verified by injection in the sibling gate
    tools/check-no-create-scene.py, whose scope is identical: a helper appended
    to each of those three left it reporting `clean`. The .md files are
    excluded on purpose, since prose is what this gate exists to preserve;
    YAML, shell and HTML are not a decision, they are a gap.
  - A ROUTE PATTERN OR ANY OTHER STRING LITERAL IS INVISIBLE, as the third
    bullet above says at length. This is the sharpest limit after the file
    types, because the route is the thing a resurrection would actually serve.
  - A JSON string VALUE is invisible. `{"name": "cellar-basics"}` in a tool
    manifest names a container without ever writing the word as a key.
  - A FILE NAME is not scanned, only file contents. client/src/view/
    pack-assets.ts, client/public/std-pack/pack.json and tools/genmappack are
    all named for the thing and all survive; a resurrected packs/ directory
    would be caught by what its files SAY, not by what they are called.
  - Inside a Python f-string the interpolated expression is masked along with
    the literal, so an identifier used only there is invisible. TypeScript
    template literals do NOT have that hole — `${...}` is unmasked and scanned
    as the code it is.
  - A REGEX LITERAL AFTER A KEYWORD IS A FALSE POSITIVE. The `/`-vs-division
    heuristic starts a regex only after one of ``(,=:[!&|?{};+-*%~^<>`` or at
    input start, so `return /pack/i.test(k);` is read as division and the word
    inside the pattern is REPORTED. Widen the prev-character set rather than
    exempting the file.
  - EVERY `Package` SPELLING ESCAPES, and it is the sharpest hole here. The
    English-word carve-out is a substring rule, so `ArtPackage`, `LoadPackage`,
    `handlePackageFile`, `PackageTile`, `packageFS`, `ErrPackageNotLoaded`,
    `packageFileURL`, `artPackages` and a `"package"` JSON key are all clean —
    proved by injection, not by reading the code. The paragraph above gives the
    measurement (308 code positions, 39 words) that makes a tighter needle
    unavailable, and names the two halves `internal/mapdef` closes instead.
    Note the self-test's test_every_spelling_of_the_english_word_is_not_a_hit
    asserts this: `packagePath` and `Subpackage` not being hits is the SAME
    assertion as `ArtPackage` not being one.
  - A JSON KEY SPELLED WITH BACKSLASH ESCAPES IS INVISIBLE. `_mask_json` keeps a
    key's raw characters, and WORD matches identifier runs, so `"\u0070ack"` —
    which every JSON parser reads as the key `pack` — contributes the word
    `u0070ack` and is not reported. Verified by injection 2026-09-06: a map file
    whose only key is spelled that way leaves this gate `clean`. _mask_json's
    own docstring has carried this since check-no-retraction.py; it belongs
    here too, because this list is the one a reader trusts to be complete.
    Nothing in this tree spells a field name in escapes, and a fixture that did
    would be doing it on purpose.
  - A RENAME WITH NO STEM defeats any pattern. `artBundle` is not this needle,
    and no gate here can say otherwise. What this one guarantees is narrower
    than the sentence that used to sit here, which claimed the shape "cannot
    come back wearing its own name, under ANY casing or separator": it cannot
    come back spelled with the STEM `pack`, under any casing or separator, in
    any of the scanned languages. Spelled `package`, it can — see the first
    bullet above.

THE MASKER BELOW IS A DELIBERATE COPY of check-no-retraction.py's, arriving
here through check-no-create-scene.py, not a shared import. That gate's
docstring argues the case and it still holds: refactoring three green gates into
a module is its own reviewed decision under CLAUDE.md rule 2, not something to
fold into the task that has to run the whole suite from cold. THE OBLIGATION
GROWS WITH THE THIRD COPY, and it is worth stating plainly rather than
inheriting quietly: a masking bug found in any of the three is a bug in all
three, and the fix has to be applied three times. Each copy is pinned by its own
boundary tests, so a divergence introduced on purpose is visible; one introduced
by fixing only one side is not.

THIS COPY IS BYTE-IDENTICAL TO check-no-create-scene.py's, comments and
docstrings included, and that is deliberate rather than incidental. Its first
draft dropped six prose hunks while keeping the code — the _mask_c_like and
_mask_json docstrings, the prev-character note, the regex-bound note, and the
interpolation note — and review found it on 2026-09-06. The cost of that is not
the missing prose: it is that a three-way `diff` then returns six hunks of
noise, inside which a genuine future divergence lands invisible, which is
exactly what the paragraph above warns about. check-no-retraction.py's copy
already differs from both in prose (its own header records corrections made
there and not propagated); that divergence predates this file and is left as
found rather than silently rewritten here.

WHEN A HIT IS CORRECT CODE, it goes in EXEMPT below with a reason, and it names
the WORDS rather than the file. Rule 2 forbids weakening a gate to pass it, so
every entry lands here, in public, with prose attached — and the self-test
asserts that no entry is whole-file, that every exempted word is something this
gate would otherwise report, and that every one of them still occurs where it is
exempted, so a dead exemption rots loudly instead of widening into a hole.

Run: python3 tools/check-no-pack.py [root]
"""

import os
import re
import sys

# A word mentions the removed container when it carries the stem `pack` NOT
# followed by `age` in the same case run. Case-aware on purpose: lower-case
# first and `packAgent` folds onto the English word and goes unreported. See
# the module docstring, and TheEnglishWordIsNotTheNeedle in the self-test.
NEEDLE = re.compile(r"[Pp]ack(?!age)|PACK(?!AGE)")

# EVERY JS/TS SPELLING, not just `.ts`. A gate whose scope is "the extension we
# happen to use today" reports clean the day somebody adds one, and closing it
# costs a line. CAVEAT CARRIED OVER FROM the two sibling gates, because it still
# applies and a future JSX author would otherwise never read it: JSX TEXT
# between tags is not a string literal to this masker, so visible copy
# containing the word would be REPORTED. No .jsx or .tsx file exists in the
# source roots today to check that against — but both are scanned, and
# test_every_javascript_and_typescript_spelling_is_scanned pins that they are.
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
# THE ANCHORING IS THE POINT. Skipping a directory called `gen` by NAME skips it
# at every depth, so `internal/foo/gen/hidden.go` — source somebody wrote, in a
# package the gate is supposed to cover — would go unscanned. Anchoring makes
# this list rot in the safe direction: a new generated tree that nobody adds
# here gets SCANNED, which is a loud false positive rather than a silent hole.
SKIP_PATHS = {
    "reports", "coverage",
    "contract/gen", "cmd/vtt/webdist",
    "contract-spike/proto/gen", "contract-spike/openapi/gen",
    "contract-spike/jsonschema/gen",
}

WORD = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")

# path -> (allowed words, or None for the whole file, reason).
#
# NONE OF THESE IS None, and the self-test asserts it: every entry names words,
# so the file around them is still scanned for every other spelling of the
# container. Two families only — the standard baseline pack, which keeps its
# name, and the sites whose job is to refuse the container. The module
# docstring argues both. Measured 2026-09-06: 87 code positions, 13 files, 23
# distinct words, and nothing outside those two families.
EXEMPT = {
    # --- the standard baseline pack, which did not leave --------------------
    #
    # client/public/std-pack/pack.json is first-party, generated by
    # tools/genmappack at this repo's own commit, shipped in the client bundle
    # and served unauthenticated from /std-pack/. It is not a container a map
    # claims and there is no route that resolves a claim against it.
    "client/src/view/pack-assets.ts": (
        {"PackManifestJSON", "PackManifestTileJSON",
         "imageRequestsForStandardPack", "loadStandardPackImages",
         "standardPackFileURL"},
        "the standard baseline's own module: manifest shape, URL builder and "
        "loader. Its header records which per-container helpers left.",
    ),
    "client/src/app.ts": (
        {"loadStandardPackImages"},
        "the boot path that fetches the baseline images, so an unoverridden "
        "square draws something",
    ),
    "client/test/pack-assets.test.ts": (
        {"PackManifestJSON", "imageRequestsForStandardPack",
         "loadStandardPackImages", "standardPackFileURL"},
        "the baseline module's own unit tests",
    ),
    "client/test/art-assets.test.ts": (
        {"loadStandardPackImages", "packAssets", "standardPackFileURL"},
        "the namespace import and the two survivors named by the ABSENCE "
        "assertion that pins which per-container helpers left the module",
    ),
    "tools/genmappack/main.go": (
        {"packFormatVersion", "packOut", "packTileOut", "writeStandardPack"},
        "the generator's on-disk shape for the baseline manifest it writes",
    ),
    "tools/genmappack/std_pack.go": (
        {"packFormatVersion", "packOut", "packTileOut", "writeStandardPack"},
        "the generator's on-disk shape for the baseline manifest it writes",
    ),
    "tools/genmappack/genmappack_test.go": (
        {"TestGeneratorReproducesTheCommittedStandardPackByteForByte",
         "packTileOut"},
        "the generator's own tests, named for the baseline they reproduce",
    ),
    "tools/genmappack/std_pack_test.go": (
        {"TestStandardPackCoversExactlyMapdefsStandardVocabulary",
         "TestStandardPackEveryEntryHasArt",
         "TestWriteStandardPackProducesElevenTilesAndAWorkingManifest",
         "writeStandardPack"},
        "the generator's own tests, named for the baseline they cover",
    ),
    # --- the sites whose job is to refuse the container ---------------------
    #
    # A gate that deleted its own enforcement would pass while proving nothing.
    "internal/mapdef/load.go": (
        {"Pack"},
        "mapJSON.Pack: the only way loadAs can refuse a map file that declares "
        "a container (design spec §7, no compatibility layer). Map itself has "
        "no such field.",
    ),
    "internal/mapdef/load_test.go": (
        {"TestAMapDeclaringAPackIsRefusedByName",
         "TestAMapDeclaringAnEmptyPackIsRefusedToo",
         "TestAMapWithNoPackFieldStillLoads",
         "TestAPackIsTheFirstThingReportedAboutAPreMigrationMap",
         "TestNoPackTypeOrLoaderRemainsInThisPackage"},
        "the refusal's own tests, plus the package-scoped absence test this "
        "gate is the tree-wide version of",
    ),
    "internal/adventure/load_test.go": (
        {"TestAnAdventureShippingAPackIsRefusedByName"},
        "the same refusal for an adventure that ships one",
    ),
    "internal/gateway/metadata_test.go": (
        {"TestNoPackRouteIsServed"},
        "the absence test for GET /api/packs/{pack}/{file}; its own doc "
        "comment names this gate as the instrument for what it cannot see",
    ),
    "cmd/vtt/art_test.go": (
        {"packish"},
        "the fixture path in TestArtInstallRefusesADirectory: installing a "
        "directory into art/ would install a container, and is refused",
    ),
}


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
    self-test can drive the per-word mechanism over a synthetic tree
    independently of what the real tree happens to need.

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
            with open(full, encoding="utf-8") as fh:
                src = fh.read()
        except (OSError, UnicodeDecodeError) as err:
            unreadable.append((rel, err.__class__.__name__))
            continue
        scanned += 1
        # Cheap pre-filter over the whole file. It cannot lose a hit a per-word
        # search would find: the needle never spans more than one word, so any
        # word that matches makes the file match too.
        if not NEEDLE.search(src):
            continue
        allowed, _ = exempt.get(rel, (set(), ""))
        if rel in exempt and allowed is None:
            continue
        code = mask(src, kind)
        for m in WORD.finditer(code):
            word = m.group(0)
            if not NEEDLE.search(word) or word in allowed:
                continue
            hits.append((rel, src.count("\n", 0, m.start()) + 1, word))
    return hits, scanned, unreadable


def main(argv):
    root = argv[1] if len(argv) > 1 else "."
    hits, scanned, unreadable = findings(root)
    if unreadable:
        print("check:no-pack: could not read %d source file(s), so they were "
              "never cleared:" % len(unreadable), file=sys.stderr)
        for rel, why in unreadable:
            print("  %s: %s" % (rel, why), file=sys.stderr)
        return 1
    if scanned == 0:
        print("check:no-pack: scanned no files under %s — the gate ran and "
              "enforced nothing" % root, file=sys.stderr)
        return 1
    if hits:
        print("check:no-pack: art is a flat library, not a container a map "
              "claims (Patrik, 2026-09-02); found %d identifier(s):" % len(hits),
              file=sys.stderr)
        for rel, line, word in hits:
            print("  %s:%d: %s" % (rel, line, word), file=sys.stderr)
        print("", file=sys.stderr)
        print("  A comment or a test string saying what a pack USED to do is "
              "fine and is not\n"
              "  reported, and so is Go's own `package` keyword and the "
              "English word. So is the\n"
              "  STANDARD BASELINE PACK, which keeps its name — it is "
              "first-party, generated by\n"
              "  tools/genmappack and served from /std-pack/, not a container "
              "any map claims.\n"
              "\n"
              "  A hit is one of THREE things, and only two of them belong in "
              "EXEMPT:\n"
              "    1. the removed container coming back — delete it;\n"
              "    2. enforcement, or the standard baseline — add the WORD to "
              "EXEMPT in\n"
              "       tools/check-no-pack.py with the reason;\n"
              "    3. ORDINARY ENGLISH that is not `package` — packet, "
              "packed, unpack, msgpack,\n"
              "       webpack, packaging. Those are deliberately NOT carved "
              "out of the needle,\n"
              "       because `unpackArt` is how a loader comes back and none "
              "of them occurred\n"
              "       at a code position when this gate was written. Widening "
              "the carve-out is\n"
              "       the honest fix for a real English word; EXEMPT is not, "
              "and filing one\n"
              "       there would put it under a family it does not belong "
              "to.", file=sys.stderr)
        return 1
    print("check:no-pack: clean (%d files scanned)" % scanned)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
