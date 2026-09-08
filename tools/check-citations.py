#!/usr/bin/env python3
"""Fail when prose cites a NAME this repository has never had.

CLAUDE.md rule 8 says citations name durable things, and names the gate it
wants: "every citation resolves to exactly one anchor, no duplicates, no
orphans — is available but unbuilt". This is the first half of that gate: the
orphan half, for Go identifiers cited in Go comments.

WHY A GATE AND NOT A REVIEW. A stale line number fails SILENTLY; so does a
fabricated symbol. `go vet` never reads a comment, the compiler never reads a
comment, and a citation to a function that does not exist looks exactly like a
citation to one that does. It is caught only by a second party who happens to
try resolving the sentence, which is not a mechanism that scales.

FOUND ON ITS FIRST RUN, over 2026-09-02-art-is-a-flat-library Task 7's own
branch, four names cited as LIVE that no commit in this repository has ever
DECLARED — three of them do appear in history, in the very commit that added
the false sentence, which is why the oracle below asks about declarations:

  - `TestSecretsAreNeverLogged` — cited as internal/identity's source-reading
    precedent. That package's real one is `TestVerifyUsesConstantTimeCompare`.
  - `TestLoadMapsDirFailsLoudWhenTheArtRootCannotBeOpened` — "pins that half".
  - `TestMetadataRulesetGuideServedWhenLoaded` — cited one line above the test
    it actually sits on, which is `...ServedForEveryRole`.
  - `TestTwoLoadsOfTheSameNewMapRaceCleanly` — "pins that refusal".

THE ORACLE IS HISTORY, AND THAT IS THE WHOLE DESIGN. "Not declared in the tree"
is not a defect on its own: this repository deliberately writes obituaries —
"X is GONE and its absence is the requirement now" — and a gate that flagged
those would be teaching people to delete the most useful prose in the tree.
What separates the two is whether the thing ever existed:

  - the name was NEVER DECLARED, in any commit -> a fabrication. Nothing to
    resolve, nothing to move it to, and the reader is sent looking for a thing
    that has never been. THIS FAILS THE GATE.
  - the name was declared once and is not declared now -> it existed and was
    deleted or renamed. An obituary is correct prose; a live claim is stale.
    The gate cannot tell those apart from the sentence, so it does not try —
    `--show-historical` lists them for a human, and they are never fatal.

`git log --all -G<declaration regex>`, NOT `-S<name>`, AND THE DIFFERENCE IS
THE GATE. `-S` (the pickaxe) counts occurrences of a STRING, so it matches the
commit that introduced the bad citation itself. Measured 2026-09-04 on the four
defects above: `-S` returns 1 commit for three of them — the commit that added
the comment — so an `-S` oracle calls three of the four "historical" and passes.
It would have caught one defect in four while reporting that it had checked.
`-G` takes a REGEX and matches added or removed LINES, so asking it for a line
that DECLARES the name ("func X", "type X", "const X", "var X") answers the
question actually being asked. Same measurement, declaration oracle: 0 commits
for all four fabrications, and >=1 for every real name and every obituary
(handlePackFile 1, LoadPack 1, ErrPackNotLoaded 2, PackTile 1).

THREE NEEDLE SHAPES, deliberately narrow, because a gate whose first run
prints a hundred false findings gets switched off:

  1. `Test[A-Z]\\w+` — a test function. Unambiguous; nothing else in prose
     looks like this.
  2. `<pkg>.<Ident>` where `<pkg>` is a REAL package directory in this module
     (mapdef.LoadPack, artlib.Lookup, gateway.Server). Precise by
     construction: the package half has to exist on disk.
  3. A bare identifier of THREE OR MORE camel segments (handlePackFile,
     WithPackFiles, ResolveObjectArt). Three, not two, and the threshold is
     measured rather than guessed: over this tree two-segment words include
     PackTile, ArtDir, LoadPack and MapMeta, which appear in ordinary English
     sentences about them, while three-segment words are essentially always a
     real identifier being pointed at.

KNOWN LIMITS, stated because a gate that overstates its coverage is worse than
one that admits a hole:

  - GO COMMENTS ONLY. Markdown, TypeScript and Python prose are not read.
    Rule 8 binds those too; this does not cover them yet.
  - It reads COMMENT text, so a citation inside a string literal is invisible.
  - Shape 3 misses a two-segment bare identifier, which is the trade above.
  - It answers "does this name exist", never "does it resolve to exactly ONE
    anchor". The duplicate half of rule 8's gate is still unbuilt.
  - It knows nothing about `[anchor:kebab-name]` strings, `file.go:123` line
    citations, or `.superpowers/` paths — three shapes rule 8 also refuses.
  - A comment that wraps MID-IDENTIFIER is recognised by is_wrap_fragment
    rather than rejoined, so a fragment that is neither a prefix nor a suffix
    of any declared name would still be read as a citation.
  - A name that exists only in a commit message, or only inside prose, reads
    as "never declared". That is the correct answer for a code citation, and
    it is what makes the gate see a fabrication that has already been
    committed once.
  - An interface METHOD and a local variable are not declaration shapes here,
    so a citation naming one reads as never declared. Neither is cited bare in
    this tree today; if that changes, widen declaration_pattern and DECL_RE
    together or the two halves will disagree. A struct FIELD is covered only
    through its wire name (WIRE_RE), not by its Go spelling.

NOT WIRED INTO `task check`. Gate wiring is apparatus work and Patrik paused
that on 2026-08-27; Task 9 of the art-is-a-flat-library plan owns gates. This
lands the script so the next apparatus block has something to switch on.

Run: python3 tools/check-citations.py [root] [--show-historical]
"""

import pathlib
import re
import subprocess
import sys

SKIP_DIRS = {"gen", "node_modules", ".git", "contract-spike", "webdist"}

# Where CITATIONS are read from is narrower than where DECLARATIONS are looked
# for, and conflating the two was this gate's first false-positive class. Go
# comments cite the TypeScript client (`pack-assets.ts's loadStandardPackImages`)
# and the generated contract, and neither tree is scanned for prose — so both
# have to be in `known` or correct sentences are reported as fabrications.
TS_GLOBS = ("client/src/**/*.ts", "client/test/**/*.ts")
CONFIG_GLOBS = (".go-arch-lint.yml", "Taskfile.yml", "tools/*.py", "tools/*.sh")

HATCH = "citations:ok"

# A declaration this tree holds: func, type, const, var, and a struct field is
# deliberately NOT included — a field is cited as pkg.Type.Field, which shape 2
# does not produce, and adding fields would widen `known` enough to hide real
# findings.
DECL_RE = re.compile(
    r"^(?:func (?:\([^)]*\) )?([A-Za-z_]\w*)"
    r"|type ([A-Za-z_]\w*)"
    r"|(?:const|var) ([A-Za-z_]\w*))")

# TypeScript's declaration shapes. `async` is optional and was missed by the
# first draft, which is how loadStandardPackImages came to be reported.
TS_DECL_RE = re.compile(
    r"^\s*(?:export\s+)?(?:default\s+)?"
    r"(?:async\s+)?(?:function|class|interface|type|const|let|var)\s+([A-Za-z_$][\w$]*)")

# A WIRE NAME, harvested from the struct tag that defines it. A protobuf field
# is generated as `AnchorFromSeq int64` with `json=anchorFromSeq` in its tag and
# travels on the wire under the lowerCamel spelling, which is what prose cites
# ("the generic dispatch's int64 anchor decode… anchorFromSeq"). A tag is the
# one place that name is written unambiguously — a struct FIELD line is not,
# because matching indented `Name Type` pairs would sweep in half of every
# composite literal in the tree.
WIRE_RE = re.compile(r"""json[=:]"?([A-Za-z_$][\w$]*)""")

TEST_RE = re.compile(r"\bTest[A-Z]\w+")
QUALIFIED_RE = re.compile(r"\b([a-z][a-z0-9]*)\.([A-Z]\w+)\b")
# Three or more camel segments, either lowerCamel or UpperCamel.
BARE_RE = re.compile(r"\b(?:[a-z][a-z0-9]*|[A-Z][a-z0-9]+)(?:[A-Z][a-z0-9]+){2,}\b")

# Names that look like our shapes but belong to Go, its toolchain, or a
# dependency — this tree cites them constantly and none is ours to declare.
# Kept as a WORD list rather than a file list, the same discipline
# check-no-create-scene.py records: exempting a file hides the next defect in it.
FOREIGN = {
    "ServeFileFS", "ServeContent", "ServeHTTP", "FileServerFS", "ServeMux",
    "ReadHeaderTimeout", "ReadDir", "ReadFile", "WriteFile", "MkdirAll",
    "OpenRoot", "DirFS", "ValidPath", "PathError", "IsNotExist", "ErrNotExist",
    "TempDir", "NewRequest", "NewRecorder", "NewServer", "StatusCode",
    "ConstantTimeCompare", "DisallowUnknownFields", "RawMessage", "MarshalIndent",
    "NewDecoder", "NewEncoder", "CombinedOutput", "ProcessState", "ExitCode",
    "WalkDir", "CutPrefix", "CutSuffix", "TrimSuffix", "HasSuffix", "SortFunc",
    "GetSceneCreated", "ProtoReflect", "NewState", "CreateInvite", "LogPath",
    "CreateImageBitmap", "LocalStorage", "SessionStorage",
}


def go_files(root):
    """Every .go file under root, tests included — a citation in a test file is
    a citation. Generated and vendored trees are skipped."""
    for path in sorted(pathlib.Path(root).rglob("*.go")):
        if SKIP_DIRS.isdisjoint(path.parts):
            yield path


def package_names(root):
    """Every directory basename that holds a .go file — the left half shape 2
    is allowed to match. Derived from the tree rather than hardcoded, so a new
    package needs no edit here."""
    return {p.parent.name for p in go_files(root)}


def source_files(root):
    """Every file whose CODE counts as evidence a name exists — a wider set than
    the files whose comments are read for citations.

    Go (generated included), the TypeScript client, and the config files Go
    comments cite by key: .go-arch-lint.yml's `mayDependOn`, Taskfile.yml's task
    names. All three are things a reader can go and find.
    """
    r = pathlib.Path(root)
    for path in sorted(r.rglob("*.go")):
        if {"node_modules", ".git", "contract-spike"}.isdisjoint(path.parts):
            yield path
    for pattern in TS_GLOBS + CONFIG_GLOBS:
        yield from sorted(r.glob(pattern))


IDENT_RE = re.compile(r"[A-Za-z_$][\w$]*")


def resolvable(paths):
    """Every identifier that appears in a CODE position somewhere in paths.

    THE QUESTION IS "CAN A READER FIND THIS", NOT "IS IT DECLARED HERE", and
    getting that wrong is what the first full run measured: a declaration-only
    set produced 157 findings over this tree, essentially all of them correct
    prose. Four causes, one answer:

      - `ListenAndServe`, `MarkFlagRequired`, `SetReadLimit` — declared by a
        DEPENDENCY (net/http, cobra, coder/websocket). Real, findable, not ours.
      - `maxDiceCount`, `errCatchUpDeadline` — declared inside a GROUPED
        `const (` / `var (` block, so no line begins with the keyword.
      - `writeFrameMu`, `fanOutDone` — struct FIELDS.
      - `mayDependOn` — a key in .go-arch-lint.yml.

    Every one of them appears in code the reader can open. Tokenising code
    positions covers all four without a Go parser, an import resolver, or a
    growing list of foreign words — and it still catches the defect this gate
    exists for, because a fabricated citation appears in PROSE AND NOWHERE ELSE.

    THE COST IS REAL AND IS ACCEPTED: a name used only as a local variable, or
    a string literal that happens to be an identifier, now counts as
    resolvable. That trades some recall for a gate that can be switched on. The
    four fabrications this branch actually shipped are all still caught.
    """
    names = set()
    for path in paths:
        try:
            src = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        code = src if path.suffix not in (".go", ".ts") else strip_line_comments(src)
        names.update(IDENT_RE.findall(code))
        for wire in WIRE_RE.findall(src):
            names.add(wire)
    return names


def strip_line_comments(src):
    """Everything left of `//` on each line. Crude — a `//` inside a string
    literal truncates that line — and safe in this direction: it can only make
    the code text SHORTER, so it never invents evidence that a name exists."""
    out = []
    for line in src.split("\n"):
        j = line.find("//")
        out.append(line if j < 0 else line[:j])
    return "\n".join(out)


def comment_text(source):
    """Every comment line's prose, as (line_number, text, block).

    `block` numbers each run of contiguous comment lines, so the hatch can
    silence a whole block the way check-doc-owner.py's does. A reason written
    once above a five-line paragraph must cover the paragraph; making an author
    repeat it per line is how a hatch turns into noise and then into a deleted
    gate.

    Line comments only. A /* */ block spanning lines would need a state machine
    and this tree does not use them — measured: zero in internal/ and cmd/.
    A `//` inside a string literal is read as a comment, which can only ADD a
    candidate, never hide one; the git oracle then clears it.
    """
    out, block, prev = [], 0, -2
    for i, line in enumerate(source.split("\n"), start=1):
        j = line.find("//")
        if j < 0:
            continue
        if i != prev + 1:
            block += 1
        out.append((i, line[j + 2:], block))
        prev = i
    return out


def candidates(text, packages):
    """Every name in text that one of the three shapes recognises."""
    found = set(TEST_RE.findall(text))
    for pkg, ident in QUALIFIED_RE.findall(text):
        if pkg in packages:
            found.add(ident)
    found |= set(BARE_RE.findall(text))
    return {n for n in found if n not in FOREIGN}


def is_wrap_fragment(name, known):
    """True when name is half of a declared name that a comment wrapped.

    Go comments wrap at 80 columns MID-IDENTIFIER, with no marker:

        // What decides the coverage is internal/artlib's TestNoLookupErrorNamesThe
        // DirectoryItRead, which walks SYSCALL PHASES ...

    reads as two candidates, `TestNoLookupErrorNamesThe` and `DirectoryItRead`,
    neither declared. Rejoining the lines is not possible without inventing a
    rule about where a space belongs, so the halves are recognised instead: a
    strict prefix or suffix of a name the tree really declares is a wrap, not a
    citation. Measured on this tree, 2026-09-04: it removes exactly that case
    and the deliberate `"...AndItsPack"` fragment, and no real finding.

    WITHOUT THIS THE GATE CAN FAIL FALSELY, which is the failure mode that gets
    a gate switched off: both halves above happen to be git-known today, so
    they land in the non-fatal bucket, but a wrap of a name coined in the
    working tree would have no history and would be reported as a fabrication.
    """
    return any(d != name and (d.startswith(name) or d.endswith(name)) for d in known)


def declaration_pattern(name):
    """A regex matching a LINE that declares name, for `git log -G`.

    Four shapes, matching DECL_RE's own: a func (with or without a receiver), a
    type, a const and a var. The trailing character class is what stops
    `func Load` from matching `func LoadPack` — without it every prefix of a
    real name reads as declared, which is precisely the class of fabrication
    this gate exists to catch.
    """
    n = re.escape(name)
    return (r"^(func([ \t]*\([^)]*\))?[ \t]+" + n + r"[ \t(\[]"
            r"|type[ \t]+" + n + r"[ \t\[]"
            r"|const[ \t]+" + n + r"[ \t=]"
            r"|var[ \t]+" + n + r"[ \t=])")


def git_knows(name, root):
    """True when some commit in this repository ever DECLARED name.

    `--all` so a name that only ever lived on a deleted branch still counts:
    the question is "has this ever existed", not "is it on main".
    """
    try:
        r = subprocess.run(
            ["git", "log", "--all", "-G", declaration_pattern(name), "--oneline", "-1"],
            cwd=root, capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.SubprocessError):
        # git is not installed, or the call blew up. See below for why YES.
        return True
    if r.returncode != 0:
        # git RAN AND COULD NOT ANSWER — root is not a repository, the object
        # store is broken, the regex was rejected. Say YES: a gate that cannot
        # consult its oracle must not invent findings, and a false PASS here is
        # recoverable where a false FAIL trains people to ignore it.
        #
        # THIS ARM IS NOT DECORATIVE, and it was written after its own test
        # failed. A non-zero exit does NOT raise — subprocess.run only raises on
        # OSError — so the earlier `return r.returncode == 0 and ...` turned
        # "not a git repository" into "declared nowhere" and reported EVERY
        # cited name in the tree as a fabrication. Found by
        # test_a_missing_git_says_known_rather_than_inventing_findings, which is
        # the whole argument for a gate having its own boundary tests.
        return True
    # Exit 0 with empty output is the real answer: git looked and found no
    # commit declaring this name.
    return r.stdout.strip() != ""


def scan(root=".", oracle=git_knows):
    """(paths, fabricated, historical).

    fabricated: (path, line, name) — cited, undeclared, in NO commit ever.
    historical: (path, line, name) — cited, undeclared, but it did exist once.
    """
    paths = list(go_files(root))
    known = resolvable(source_files(root))
    packages = package_names(root)
    fabricated, historical, verdict = [], [], {}
    for path in paths:
        lines = comment_text(path.read_text(encoding="utf-8"))
        hatched = {b for _, t, b in lines if HATCH in t}
        for line_no, text, block in lines:
            if block in hatched:
                continue
            for name in sorted(candidates(text, packages)):
                if name in known or is_wrap_fragment(name, known):
                    continue
                if name not in verdict:
                    verdict[name] = oracle(name, root)
                (historical if verdict[name] else fabricated).append(
                    (str(path), line_no, name))
    return paths, fabricated, historical


def report(path, line, name):
    return (
        f"check:citations: {path}:{line}: cites `{name}`, which this repository has "
        f"NEVER declared — no commit reachable from any ref adds or removes a line "
        f"declaring it (`git log --all -G` for a func/type/const/var line is empty). A "
        f"reader following this sentence "
        f"has nothing to find, and the citation still LOOKS valid, which is the whole "
        f"defect CLAUDE.md rule 8 names. Either correct it to the real name or delete "
        f"the claim. If the name is a DELIBERATE counter-example — prose that says "
        f"\"X, not Y\" about a Y that must not exist — adjudicate it with a "
        f"`//{HATCH} <reason>` line rather than rewording, the same hatch "
        f"check-doc-owner.py uses."
    )


def main(argv):
    args = [a for a in argv[1:] if not a.startswith("--")]
    show_historical = "--show-historical" in argv
    root = args[0] if args else "."
    paths, fabricated, historical = scan(root, oracle=git_knows)
    if not paths:
        print(
            f"check:citations: no Go files under {root} — nothing was scanned, so nothing "
            f"is proven. A clean run over an empty tree is not a pass.",
            file=sys.stderr,
        )
        return 2
    for finding in fabricated:
        print(report(*finding), file=sys.stderr)
    if show_historical:
        for path, line, name in historical:
            print(f"check:citations: (historical) {path}:{line}: `{name}` is not in the "
                  f"tree but did exist — an obituary is correct prose, a live claim is stale.")
    if fabricated:
        return 1
    print(f"check:citations: {len(paths)} files, every cited name has existed "
          f"({len(historical)} cite something the tree no longer declares).")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
