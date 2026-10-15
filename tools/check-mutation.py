#!/usr/bin/env python3
"""Fail on any surviving mutant that has not been adjudicated equivalent.

Why a survivor list rather than gremlins' own --threshold-efficacy
-----------------------------------------------------------------
gremlins gates itself on an efficacy PERCENTAGE, which is a ratio: enough
newly-killed mutants can mask a new survivor. internal/engine at 49 killed /
2 lived is 96.08%; at 100 killed / 3 lived it is 97.1% — a real new gap,
passing a 96.0 floor. The 2026-07-24 mutation audit's own standard was "zero
unadjudicated survivors", and that is what this enforces: every survivor must
be named in tools/mutation-equivalents.txt with a reason, or the gate fails.

Patrik chose this over the toolchain threshold (2026-07-28). The precision is
worth the hand-rolled parsing: a masked survivor is exactly the failure this
gate exists to prevent.

What this deliberately does NOT do
----------------------------------
It does not implement mutation testing or schedule it. gremlins does all of
that. This reads its output and compares one set against another — the part
gremlins has no opinion about. That boundary is deliberate: across five review
rounds of the coverage gate, 8 of 9 defects were in hand-rolled enforcement
code, and the fix was always to delete it in favour of the toolchain. Keep this
file small enough that it cannot hide a bug.

IT READS GO SOURCE, as lines of bytes and nothing more. An adjudication names
the code its mutant sits on, not a line (SPEC-020), so the gate reads the line
gremlins names for each survivor, from the package as it stood when gremlins
started, and compares the trimmed text and the offset in it with each entry's.
Before the run it checks each entry against the tree: its code stands in its
file as often as the entry records, and its offset holds a token its mutator
rewrites. No parser, no AST, no opinion about what the code means.
"""

import collections
import hashlib
import json
import os
import re
import shutil
import tempfile
import subprocess
import sys

# Every package whose mutation run is feasible. This list was ONCE the five
# packages that happened to have been measured while writing the gate — which
# silently dropped internal/campaign, a package the older audit:mutation task
# DID cover and which owns the fold driver, undo, and the poison contract.
# Gating it immediately found three real survivors in c.head's batch
# arithmetic. A gate scoped to what its author touched is not a gate.
#
# internal/harness is STILL excluded, and the reason has changed twice. Keep
# the history, because each version was true when written and each was
# superseded by doing the work rather than by argument.
#
#  1. "Fixed sleeps cost ~70s PER MUTANT, so a run is hours." VOID since
#     2026-07-29: the tests moved into testing/synctest bubbles, the suite
#     went 51s -> 0.7s, and a full run is under four minutes.
#  2. "29 unadjudicated survivors." Worked down to 2 across five passes.
#  3. The actual blocker now: the last real survivor is soak.go:271's
#     CONDITIONALS_NEGATION, which removes a guard that exists to close a
#     RACE — the "has every participant drained the last accepted event"
#     wait before a denial snapshot. A test CAN detect its removal (see
#     TestSoakSlowBroadcastIsNotMistakenForALeak, which delays the fan-out to
#     force the ordering) but only ~60% of the time, measured over repeated
#     runs. The test is stable on correct code, 8/8; it is the DETECTION that
#     is probabilistic.
#
# That is the honest shape of it: synctest made mutation testing feasible
# here, and the same fake clock makes race-guard mutants nondeterministic to
# kill, because "all goroutines durably blocked" erases the very interleaving
# the guard defends against. Gating on a ~60% kill would hand this repo a
# flaky gate, which is the failure mode every other gate here was built to
# avoid. soak.go:611 compounds it: it flips between LIVED and TIMED OUT
# across runs, so even an adjudication for it goes stale at random.
#
# Adding harness needs the race guard's effect made deterministic — not more
# survivors killed.
#
# What is NOT here, with measured survivor counts, is published in
# tools/mutation-scope.md — so the narrowness is a number rather than an
# impression. Three packages turned out to be outside this list on 2026-08-04
# with no recorded reason at all. All three -- internal/rules/conformance,
# internal/adventure and internal/rules -- have since been worked to zero
# unadjudicated survivors and gated. NONE remains outside on no argument.
# cmd/vtt is excluded on the record by ADR-010:96-97, and carries a SECOND,
# independent blocker found 2026-08-05: its scenario tests boot against a
# directory whose only entry is a symlink, and gremlins' workdir copy drops
# symlinks silently — see dropped_symlinks() below. Resolving the package name
# does NOT make it gateable; both routes produce the same constant KILLED
# verdict, and its published "no genuine survivors among the 77 evaluated" was
# retracted for exactly that reason.
#
# tools/toolgen was REMOVED from this list on 2026-08-04. It is `package main`
# in a directory not named `main`, which gremlins cannot resolve — see
# unresolvable_packages() below — so every one of its mutants had been scored
# a false kill for as long as it had been gated. Measured by hand in a renamed
# worktree it is genuinely 9 killed / 0 lived, but the gate was not producing
# that answer and a green line that measures nothing is worse than a gap.
PACKAGES = [
    # Ascending by runtime: the gate fails as fast as it can.
    "./internal/identity/",                # ~16s
    "./internal/adventure/conformance/",   # ~19s
    # internal/artlib (art-is-a-flat-library): pure path and format validation
    # over a directory, and the package every art read in the tree goes
    # through -- exactly what a mutation gate is best at, and gated from its
    # FIRST commit rather than after a review found it missing, which is what
    # happened to internal/mapdef below.
    "./internal/artlib/",                  # ~30s
    # internal/campaigncfg (art-is-a-flat-library Task 6): one file reader whose
    # whole body is a chain of decisions about what a hand-written JSON file may
    # and may not say — absent, present-and-broken, a version this server does
    # not understand, a cell size that is not a positive whole number. That is
    # exactly what a mutation gate is best at, and it is gated from its FIRST
    # commit rather than after a review finds it missing, which is what happened
    # to internal/mapdef below.
    "./internal/campaigncfg/",             # ~13s
    # internal/mapdef (maps-as-geometry): the arc's core package, and pure
    # validation logic — exactly what a mutation gate is best at. It was in
    # NEITHER this list nor mutation-scope.md, whose stated job is to publish
    # what sits outside the gate, so it was silently ungated rather than
    # deliberately excluded. Found by the whole-branch review.
    "./internal/mapdef/",
    "./internal/store/",                   # ~33s
    "./internal/engine/",                  # ~38s
    "./internal/rules/conformance/",       # ~48s
    "./internal/gateway/",                 # ~57s
    "./internal/adventure/",               # ~60s
    # internal/perceive: the perception decision, moved out of internal/gateway
    # with its own tests (docs/superpowers/plans/2026-10-15-the-perception-decision-leaves-the-gateway.md).
    # One timeout is expected, a non-terminating objectInSight loop (tools/mutation-scope.md).
    "./internal/perceive/",                # ~87s
    "./internal/campaign/",                # ~91s
    # internal/sight (visibility): pure geometry with no I/O — exactly what a
    # mutation gate is best at, and gated from its FIRST commit. internal/mapdef
    # was in neither this list nor mutation-scope.md, so it shipped silently
    # ungated; adding it later found 13 live mutants and one real bug.
    #
    # 190-335s across runs for only ~106 mutants, which is out of proportion to
    # the package and is worth knowing before anyone tries to speed it up: FOUR
    # of them are INCREMENT_DECREMENT on the grid-walk loop counters and CANNOT
    # TERMINATE, so each burns a full deadline rather than failing. The deadline
    # is baseline x TIMEOUT_COEFFICIENT, so it moves with how warm the cache was
    # AND with the suite's own runtime — which is why the spread is over two
    # minutes on code that only grew by a third. See tools/mutation-scope.md on
    # why those four, with internal/perceive's one, are the honest cases in this
    # repo for counting a timeout as a detection.
    "./internal/sight/",                   # ~335s, much of it four unkillable timeouts
    "./internal/rules/",                   # ~393s -- the interpreter, 553 mutants
    "./internal/mcp/",                     # ~857s
]

# `LIVED CONDITIONALS_BOUNDARY at apply.go:147:15`
# The floor below which a mutation run must not START.
#
# MEASURED 2026-08-10 on this machine, with a freshly emptied cache: one full
# `task check` consumed 7.6 GiB of free space and left GOCACHE 4 GiB larger than
# it found it. gremlins copies the whole tree per mutant into $TMPDIR and every
# mutant is a distinct binary, so the cache grows without bound across runs —
# which is how it reached 82 GB and filled a 228 GB volume.
#
# 16 GiB is that measured cost with room for one more run, not a round number
# picked for looking careful. It is deliberately generous: the cost of refusing
# a run that would have squeaked through is a clear message and a `go clean
# -cache`, while the cost of admitting one that cannot finish is a CORRUPT
# VERDICT that looks exactly like a good one — and, when the volume actually
# fills, a machine on which Bash itself stops working, so the failure does not
# even present as a gate failure.
MIN_FREE_BYTES = 16 * 1024**3

# One full gate run's measured consumption of free space, rounded up from the
# 7.6 GiB recorded above. Named so the warning below is DERIVED from it rather
# than being a second independent guess about the same run.
RUN_COST_BYTES = 8 * 1024**3

# Where the disk gets a WORD, well before MIN_FREE_BYTES gets a veto.
#
# The floor fails closed, but only at a cliff, and by then the volume may be too
# full for Bash itself to work. This is the ramp: three more runs' worth of room
# above the floor, so there is time to act while acting is still easy.
#
# TRIGGERED ON FREE SPACE, NOT ON CACHE SIZE, which was a review finding rather
# than the first design. GOCACHE is MACHINE-GLOBAL — every Go project on the box
# shares it — so "the cache is over N GiB" says nothing about whether THIS
# volume is in trouble. A developer with two other Go repos would sit above any
# useful threshold permanently with 100 GiB free, and be told every single run
# to `go clean -cache`, throwing away every project's cache to solve nothing. A
# warning that fires every time is one people learn to scroll past, which costs
# the warning that mattered. Free space is the quantity that actually ran out;
# cache size is a proxy that tracks it only on a small volume.
#
# ADVISORY ONLY. It never changes the exit code. Making it refuse would be a new
# gate, and a gate change is its own reviewed decision (CLAUDE.md rule 2).
CACHE_WARN_FREE_BYTES = MIN_FREE_BYTES + 3 * RUN_COST_BYTES


def go_cache():
    """GOCACHE, from the environment or the toolchain.

    Returns "" if the toolchain reports none. RAISES if `go` is not on PATH,
    deliberately: swallowing that would make _disk_targets silently drop GOCACHE
    from the floor's targets, narrowing a gate by accident.
    """
    return os.environ.get("GOCACHE") or subprocess.run(
        ["go", "env", "GOCACHE"], capture_output=True, text=True).stdout.strip()


def cache_size(path):
    """Bytes under path, or None if that cannot be measured.

    Only ever called to enrich a warning that has ALREADY been decided, so None
    costs one clause of a message rather than the message.

    du rather than an os.walk: the build cache holds hundreds of thousands of
    small files. Measured 0.09s at 4.31 GiB warm; ~25s cold on a 450k-file tree.

    THE EXIT STATUS IS NOT CONSULTED, only stdout. Review measured a real
    450k-file cache where du exited 1 over nine unreadable entries while
    printing a perfectly good total, and a file vanishing mid-walk does the
    same — not exotic here, since gopls and any concurrent `go` command write
    and trim this directory continuously. A partial total can only UNDERSTATE,
    so parsing one risks a warning that is too quiet, while discarding it risks
    no warning at all.
    """
    if not path or not os.path.isdir(path):
        return None
    try:
        proc = subprocess.run(["du", "-sk", path], capture_output=True,
                              text=True, timeout=120)
    except (OSError, subprocess.SubprocessError):
        return None
    try:
        return int(proc.stdout.split()[0]) * 1024
    except (IndexError, ValueError):
        return None



# A run that could not APPLY or RESTORE a mutation is not a measurement.
#
# gremlins copies the tree per mutant into $TMPDIR. When the disk filled it
# emitted one of these per mutant, then printed a normal summary and EXITED 0 —
# so the gate produced confident, precise numbers from a run that had measured
# something other than what it claimed.
#
# The two halves fail differently and both are silent:
#   failed to APPLY   — the mutation was never written, so the test ran against
#                       CLEAN SOURCE and the verdict describes unmutated code.
#   failed to RESTORE — the mutation stays in the workdir file, so EVERY
#                       SUBSEQUENT mutant in that run is measured against
#                       already-corrupted source.
#
# Anchored on gremlins' own phrasing rather than on the word ERROR, which a
# package's own tests may legitimately log — a guard that reds the gate on
# those gets weakened, and a weakened gate is how this started.
# gremlins' real shape, read from the source rather than guessed:
#
#   log.Errorf -> eWritef("%s: %s", ...)   -- Writef, NO TRAILING NEWLINE
#   executor.go:178 -> "failed to apply mutation at %s - %s\n\t%v"  -- a TAB
#
# So the cause is tab-indented, and because the error itself never ends a line,
# CONSECUTIVE ERRORS GLUE onto the previous one's cause. A run with 88 failures
# is not 88 tidy lines. An earlier version of this anchored on `^` and looked
# for a `...` continuation — a shape gremlins cannot produce — and matched
# exactly ONE of them, dropping "no space left on device" with it. The phrase
# is the anchor; the line start is not.
# The PHRASE ONLY, so the count is a count. Matching the cause line too made a
# single match swallow the next glued error, and 88 failures reported as 1.
MUTATION_IO_ERROR_RE = re.compile(r"failed to (?:apply|restore) mutation\b")

# Every per-mutant verdict line gremlins prints, cross-checked against the
# summary counts.
#
# WHAT THIS DOES AND DOES NOT CATCH, because the first version of this comment
# claimed the wrong thing. It does NOT catch a disk-exhausted run: the printed
# lines and the summary come from the same slice (engine.go:229), and a mutant
# that failed to apply is dropped from both, so the two agree by construction
# however corrupt the run. The errors above are what catch that.
#
# What it DOES catch is a printed set that no longer matches the counted set —
# output filtering (see --output-statuses, now pinned) and format drift. It is
# the check that would have caught `GREMLINS_UNLEASH_OUTPUT_STATUSES=k`
# silencing every survivor: 74 lines against 75 counted.
MUTANT_LINE_RE = re.compile(
    r"^\s*(?:KILLED|LIVED|TIMED OUT|NOT COVERED|NOT VIABLE|SKIPPED)\s+\S+\s+at\s+\S+\s*$",
    re.M)

SUMMARY_COUNTS_RE = re.compile(
    r"^Killed:\s*(\d+),\s*Lived:\s*(\d+),\s*Not covered:\s*(\d+)\s*$"
    r"\s*^Timed out:\s*(\d+),\s*Not viable:\s*(\d+),\s*Skipped:\s*(\d+)\s*$", re.M)

LIVED_RE = re.compile(r"^\s*LIVED\s+(\S+)\s+at\s+(\S+)\s*$")

# `   TIMED OUT CONDITIONALS_NEGATION at server.go:158:9`
TIMED_OUT_RE = re.compile(r"^\s*TIMED OUT\s+(\S+)\s+at\s+(\S+)\s*$")

# `Timed out: 58, Not viable: 0, Skipped: 0`
TIMEOUT_RE = re.compile(r"^Timed out:\s*(\d+)", re.M)
KILLED_RE = re.compile(r"^Killed:\s*(\d+),\s*Lived:\s*(\d+)", re.M)

# A timed-out mutant is one gremlins abandoned because the suite ran too long
# under it. Patrik's ruling (2026-07-28): count those as detections rather than
# failing the gate on them, since in this codebase they are overwhelmingly
# mutants that hang a socket or a wait.
#
# BUT THEY ARE NOT RELIABLY DETECTIONS, and an earlier version of this comment
# claimed they were ("the suite did not pass"). A review disproved it with a
# working counterexample: a lenient assertion over a value whose upper bound
# the mutation removes —
#
#     if d > ceiling { d = ceiling }   // mutate away the clamp
#     time.Sleep(d)                    // now sleeps 60s
#     ... test asserts only `got >= 0` ...
#
# gremlins reports TIMED OUT; applying that same mutation by hand, the suite
# PASSES in 60s. The mutant genuinely survives and this gate would score it as
# detected. The shape is not exotic — deadlines, backoffs and handshake
# timeouts are exactly what the gated packages are full of.
#
# So every timed-out mutant is PRINTED with its location. They are not failures
# per the ruling, but they are the set this gate did not actually measure, and
# discarding them is how a survivor disappears silently. gremlins names each
# one; there is no excuse for the gate to know less than its own input.
#
# The cap is the second guard. Counting timeouts as kills inverts into the
# failure observed here first: two goroutine-waiting tests took internal/mcp
# from 57 killed / 8 lived / 0 timed out to 6 / 0 / 58, with gremlins still
# printing "Test efficacy: 100.00%" (it computes killed/(killed+lived) and
# ignores timeouts). Scoring that as perfect would be worse than useless.
# Measured proportions, two orders apart:
#     mcp (broken)   58 of 64 timed out = 91%
#     gateway         6 of 33 timed out = 18%
#     mcp (fixed)     0 of 64 timed out =  0%
# A MAJORITY fails as an unusable measurement. The denominator is
# killed+lived+timed_out; "not covered" mutants are excluded by definition,
# since no test reaches them. Exactly 50% is treated as acceptable — the
# boundary is pinned by test, not left to the reader.
MAX_TIMEOUT_FRACTION = 0.50


class EquivalentsError(Exception):
    pass


# An adjudication names the code it excuses, never its line (SPEC-020):
#
#     <package>  <file>  <MUTATOR>  offset=<n>  nth=<n>  copies=<n>
#         code: <the line the mutant sits on, trimmed>
#         <reason>
#
# `offset` is gremlins' byte column less the line's indentation, so a line that
# moves or is re-indented keeps its entry and a line that is edited loses it.
# `copies` is how many lines of the file hold that code, and `nth` which of them
# the entry names, counted from the top: an entry on one copy never excuses a
# mutant on another.
Entry = collections.namedtuple("Entry", "package file mutator offset nth copies code reason")

# The reason the gate prints under a ready entry. Refused as a reason: pasted
# unedited, it would excuse a mutant nobody judged.
PLACEHOLDER = "<the observable that makes it equivalent>"


def entry_key(e):
    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)


def describe(e):
    return f"{e.package} {e.file} {e.mutator} offset={e.offset} nth={e.nth} on `{e.code}`"


def read_equivalents(path):
    """[Entry], in file order.

    A line whose FIRST character is `#` is a comment; a `#` anywhere else is
    text. Under each entry come its code line and its reason, both required:
    an equivalence claim without a stated observable is the shape that
    produced two wrong adjudications on 2026-07-27.
    """
    want = "want '<package>  <file>  <MUTATOR>  offset=<n>  nth=<n>  copies=<n>', each <n> at least 1"
    entries, head, code = [], None, None
    with open(path) as fh:
        for lineno, raw in enumerate(fh, 1):
            line = raw.rstrip("\r\n")
            if not line.strip() or line.startswith("#"):
                continue
            if line[0].isspace():
                text = line.strip()
                if head is None:
                    raise EquivalentsError(f"{path}:{lineno}: an indented line with no entry above it")
                if code is None:
                    # Trimmed as the gate trims source (INDENT), so a printed
                    # entry pasted back names the same code.
                    text = line.lstrip(" \t").rstrip(" \t\r")
                    if not text.startswith("code: "):
                        raise EquivalentsError(
                            f"{path}:{lineno}: entry {head[1]} {head[2]} has no code line — the "
                            f"first line under an entry is `code: <the line its mutant sits on, "
                            f"trimmed>`")
                    code = text[len("code: "):]
                    continue
                if text == PLACEHOLDER:
                    raise EquivalentsError(
                        f"{path}:{lineno}: entry {head[1]} {head[2]} has the printed placeholder for "
                        f"its reason — state the observable that makes it equivalent, or write the "
                        f"test instead")
                entries.append(Entry(*head, code, text))
                head, code = None, None
                continue
            if head is not None:
                raise EquivalentsError(_unfinished(path, lineno, head, code))
            parts = line.split()
            try:
                if len(parts) != 6 or [x.split("=")[0] for x in parts[3:]] != ["offset", "nth", "copies"]:
                    raise ValueError
                offset, nth, copies = (int(x.split("=", 1)[1]) for x in parts[3:])
                if min(offset, nth, copies) < 1:
                    raise ValueError
            except ValueError:
                raise EquivalentsError(f"{path}:{lineno}: {want}, got {line.strip()!r}") from None
            if parts[0] != _norm_pkg(parts[0]):
                raise EquivalentsError(
                    f"{path}:{lineno}: write the package as survivors carry it, "
                    f"{_norm_pkg(parts[0])!r}, not {parts[0]!r}: as written it matches nothing")
            head = (parts[0], parts[1], parts[2], offset, nth, copies)
    if head is not None:
        raise EquivalentsError(_unfinished(path, "end of file", head, code))
    return entries


def _unfinished(path, where, head, code):
    if code is None:
        return (f"{path}:{where}: entry {head[1]} {head[2]} has no code line — the first line "
                f"under an entry is `code: <the line its mutant sits on, trimmed>`")
    return (f"{path}:{where}: entry {head[1]} {head[2]} has no reason line — state the observable "
            f"that makes it equivalent, or write the test instead")


def _norm_pkg(name):
    """A package as survivors carry it: `internal/x`, never `./internal/x/`."""
    return name.strip("./").rstrip("/")


# The operators each mutator rewrites. An entry whose offset holds none of its
# mutator's tokens names no mutant gremlins makes, so it could only go stale.
#
# token_at is a byte match with no parser behind it: the `<` of a `<-` passes as
# CONDITIONALS_BOUNDARY, and a pointer `*` as ARITHMETIC_BASE. This catches the
# common mistake, not every one.
#
# gremlins' columns are 1-BASED BYTE OFFSETS, and Go source is TAB-indented: a
# tab is one byte, never a tab stop.
MUTATOR_TOKENS = {
    "CONDITIONALS_BOUNDARY": ("<=", ">=", "<", ">"),
    "CONDITIONALS_NEGATION": ("==", "!=", "<=", ">=", "<", ">"),
    "ARITHMETIC_BASE": ("+", "-", "*", "/", "%"),
    "INCREMENT_DECREMENT": ("++", "--"),
    "INVERT_NEGATIVES": ("-",),
    "INVERT_LOGICAL": ("&&", "||"),
}

# `//`, `/*`, and a line whose first non-space is `*` — the middle of a block
# comment. Test the third: a check of `//` alone misses `/**` and its
# continuation lines.
COMMENT_PREFIXES = (b"//", b"/*", b"*")

INDENT = b" \t\r"


def trimmed(raw):
    """(code, width of the indentation) of one source line, in bytes."""
    return raw.strip(INDENT), len(raw) - len(raw.lstrip(INDENT))


def token_at(line, col, tokens):
    """The longest of `tokens` starting at 1-based BYTE column `col`, or "".

    Longest-first so `<=` is not read as a bare `<`: they are different
    mutations.
    """
    if col < 1 or col > len(line):
        return ""
    tail = line[col - 1:]
    for tok in sorted(tokens, key=len, reverse=True):
        if tail.startswith(tok.encode()):
            return tok
    return ""


def package_source(root, pkg):
    """{file: [line bytes]} for the .go files under the package's directory as
    they stand now, each named by its path relative to the package, since
    gremlins recurses into ungated subdirectories and names their files so.

    Read BEFORE gremlins runs: its positions name the source it mutated, and the
    tree can be edited while it runs (VTT-303).
    """
    d = os.path.join(root, _norm_pkg(pkg))
    source = {}
    for dirpath, _, names in os.walk(d):
        for name in names:
            if name.endswith(".go"):
                path = os.path.join(dirpath, name)
                try:
                    with open(path, "rb") as fh:
                        source[os.path.relpath(path, d).replace(os.sep, "/")] = fh.read().split(b"\n")
                except OSError:
                    pass
    return source


def resolve(source, name, location, mutator):
    """The key of the mutant gremlins names at `file:line:col`, or None when its
    line cannot be read from `source` or its column falls outside the code."""
    parts = location.rsplit(":", 2)
    if len(parts) != 3:
        return None
    file_, line, col = parts
    try:
        line, col = int(line), int(col)
    except ValueError:
        return None
    lines = source.get(file_)
    if lines is None or not 1 <= line <= len(lines):
        return None
    code, indent = trimmed(lines[line - 1])
    try:
        text = code.decode("utf-8")
    except UnicodeDecodeError:
        return None
    if not text or not 1 <= col - indent <= len(code):
        return None
    nth = sum(1 for raw in lines[:line] if trimmed(raw)[0] == code)
    return (name, file_, mutator, col - indent, nth, text)


def copies_in(source, file_, code):
    """How many lines of `file_` in `source` hold `code`."""
    want = code.encode("utf-8")
    return sum(1 for raw in source.get(file_, ()) if trimmed(raw)[0] == want)


def entry_faults(entries, root="."):
    """[(Entry, fault)] for every entry the tree refutes, before any mutation run.

    Refused: a file that cannot be read; code that stands in its file a number
    of times other than `copies` (VTT-302); an `nth` above `copies`; two
    entries for one key; code that is a comment; an offset holding no token its
    mutator rewrites; a mutator this check cannot place (fail closed).
    """
    sharing = collections.Counter(entry_key(e) for e in entries)
    files = {}
    bad = []
    for e in entries:
        path = os.path.join(root, e.package, e.file)
        if path not in files:
            try:
                with open(path, "rb") as fh:
                    files[path] = [trimmed(raw)[0] for raw in fh.read().split(b"\n")]
            except OSError:
                files[path] = None
        fault = _entry_fault(e, path, files[path], sharing[entry_key(e)])
        if fault:
            bad.append((e, fault))
    return bad


def _entry_fault(e, path, lines, sharing):
    if lines is None:
        return f"{path} cannot be read"
    code = e.code.encode("utf-8")
    n = lines.count(code)
    if n != e.copies:
        return f"`{e.code}` stands {n} time(s) in {path}, and the entry records copies={e.copies}"
    if e.nth > e.copies:
        return f"it names copy nth={e.nth} of code that stands {e.copies} time(s) in {path}"
    if sharing > 1:
        return (f"{sharing} entries name this code, copy, mutator and offset, and one mutant can "
                f"be excused only once")
    if code.startswith(COMMENT_PREFIXES):
        return "its code is a COMMENT, and gremlins generates no mutant on one"
    tokens = MUTATOR_TOKENS.get(e.mutator)
    if tokens is None:
        return (f"{e.mutator} has no entry in MUTATOR_TOKENS, so this check cannot say which token "
                f"it applies to and is not checking this entry at all. Add it with the operators "
                f"gremlins rewrites for it")
    if not token_at(code, e.offset, tokens):
        got = code[e.offset - 1:e.offset + 7].decode("utf-8", "replace") if e.offset <= len(code) else ""
        return (f"offset {e.offset} of {e.code!r} holds {got!r}, and {e.mutator} applies only to "
                f"{' '.join(tokens)}")
    return ""


def claims(entries, survivors, timed_out):
    """(claimed, unadjudicated, stale): entries matched to live mutants by key
    (SPEC-020).

    `survivors` and `timed_out` are [(key, mutant)], `mutant` being however the
    caller names one. Per key, survivors are claimed first and one per entry
    (VTT-300), in the order given; entries left over claim timed-out mutants,
    since a timeout is a detection and not an evaluation; entries left after
    that are stale (VTT-301). `unadjudicated` is [(key, mutant)].
    """
    groups = collections.defaultdict(lambda: ([], [], []))
    for e in entries:
        groups[entry_key(e)][0].append(e)
    for key, mutant in survivors:
        groups[key][1].append(mutant)
    for key, mutant in timed_out:
        groups[key][2].append(mutant)
    claimed, unadjudicated, stale = [], [], []
    for key, (named, lived, hung) in groups.items():
        n = min(len(named), len(lived))
        claimed += lived[:n]
        unadjudicated += [(key, m) for m in lived[n:]]
        k = min(len(named) - n, len(hung))
        claimed += hung[:k]
        stale += named[n + k:]
    return claimed, unadjudicated, stale


def _by_position(item):
    """Sort key of a (key, (location, mutator)) pair: file, line, column."""
    file_, line, col = item[1][0].rsplit(":", 2)
    return (file_, int(line), int(col))


def _first_io_error(out):
    """The first apply/restore error WITH its cause, as one readable line.

    Cut at the next `ERROR:` because they glue: gremlins' Errorf writes no
    trailing newline, so a sibling error starts partway through the previous
    one's tab-indented cause.
    """
    m = MUTATION_IO_ERROR_RE.search(out)
    if not m:
        return ""
    tail = out[m.start():m.start() + 400]
    nxt = tail.find("ERROR:", 1)
    if nxt != -1:
        tail = tail[:nxt]
    return " ".join(tail.split())


def parse_survivors(output):
    """Return [(location, mutator)] from one gremlins run's output."""
    survivors = []
    for line in output.splitlines():
        m = LIVED_RE.match(line)
        if m:
            survivors.append((m.group(2), m.group(1)))
    return survivors


def parse_timed_out(output):
    """Return [(location, mutator)] for mutants gremlins abandoned.

    These are counted as detections per the ruling above, but they are NOT
    reliably so — see MAX_TIMEOUT_FRACTION's comment for the counterexample.
    They are the set this run did not actually measure, so they get printed.
    """
    out = []
    for line in output.splitlines():
        m = TIMED_OUT_RE.match(line)
        if m:
            out.append((m.group(2), m.group(1)))
    return out


def run_package(pkg, runner):
    """Run gremlins over pkg; return (survivors, timed_out).

    Raises if the run did not complete, or if so many mutants timed out that
    the result says nothing about the ones that did not.
    """
    out = runner(pkg)

    # BEFORE anything is parsed. A run carrying these errors has no verdict to
    # extract, and reading one out of it is the defect: the numbers look exactly
    # like a good run's.
    broken = MUTATION_IO_ERROR_RE.findall(out)
    if broken:
        raise EquivalentsError(
            f"{pkg}: gremlins could not apply or restore {len(broken)} mutation(s), so this run "
            f"is NOT a measurement and no verdict may be read from it. A failed APPLY means the "
            f"test ran against CLEAN source; a failed RESTORE means every subsequent mutant in "
            f"the run was measured against already-corrupted source. The usual cause is a full "
            f"disk — gremlins copies the tree per mutant into $TMPDIR, and `task check:mutation` "
            f"grows GOCACHE without bound (82 GiB measured). Check free space, `go clean -cache`, "
            f"and run it again. First error:\n    {_first_io_error(out)}")

    # gremlins exits non-zero when mutants survive and no threshold is set, so
    # the exit code is not a failure signal here — the survivor list is.
    if "Mutation testing completed" not in out:
        raise EquivalentsError(
            f"gremlins produced no summary for {pkg} — it did not complete:\n{out}")

    tm = TIMEOUT_RE.search(out)
    km = KILLED_RE.search(out)
    if not tm or not km:
        # Failing open here would re-create the exact blind spot this gate was
        # built after: a grep that never looked at the `Timed out:` line
        # reported two packages as "100% efficacy" while a quarter to a
        # majority of their mutants went unevaluated.
        raise EquivalentsError(
            f"{pkg}: could not parse gremlins' summary counts (Killed/Lived/Timed out) — "
            f"the output format changed, so the timeout check cannot run and this result "
            f"cannot be trusted. gremlins is pinned at v0.6.0 for this reason; check the pin.")
    # The summary must account for every per-mutant line above it. Generic
    # where the check above is specific: it catches silent disappearance
    # whatever the cause, and it costs one regex.
    sm = SUMMARY_COUNTS_RE.search(out)
    if not sm:
        # FAIL CLOSED, like the counts check twelve lines above. Skipping the
        # tally on an unrecognised summary would put the gate back where it
        # started: quietly not looking.
        raise EquivalentsError(
            f"{pkg}: gremlins' summary lines did not parse, so the per-mutant tally cannot be "
            f"checked and this result cannot be trusted. The output format changed; gremlins is "
            f"pinned at v0.6.0 for exactly this reason, so check the pin.")
    claimed = sum(int(g) for g in sm.groups())
    printed = len(MUTANT_LINE_RE.findall(out))
    if printed == 0:
        # Every gated package has hundreds of mutants. Zero is not a clean
        # sweep, it is a run that measured nothing — a mis-scoped
        # --exclude-files, or a package that failed to parse.
        raise EquivalentsError(
            f"{pkg}: gremlins printed no per-mutant verdicts at all. That is a run which "
            f"measured nothing, not a package with nothing to measure — check --exclude-files "
            f"and that the package builds.")
    if claimed != printed:
        raise EquivalentsError(
                f"{pkg}: gremlins printed {printed} per-mutant line(s) but its summary accounts "
                f"for {claimed}. Mutant count is a pure function of the parsed source, so a "
                f"summary that does not add up is proof that mutants went missing — the result "
                f"cannot be trusted whatever the cause. A disk-exhausted run tallied 467 of 555 "
                f"this way and reported a clean efficacy.")

    timed_out = int(tm.group(1))
    evaluated = int(km.group(1)) + int(km.group(2))
    total = evaluated + timed_out
    if total > 0 and timed_out / total > MAX_TIMEOUT_FRACTION:
        raise EquivalentsError(
            f"{pkg}: {timed_out} of {total} mutants TIMED OUT ({timed_out / total:.0%}) — "
            f"a majority, so this is a broken measurement rather than detection, and "
            f"gremlins would still report a clean efficacy (it computes killed/(killed+"
            f"lived) and ignores timeouts). A test that BLOCKS on a long deadline is one "
            f"cause and the only one a test change fixes — dropping two mcp deadlines from "
            f"10s to 3s took that package from 58 timeouts to 1. A deadline collapsed by a "
            f"warm coverage cache and a starved machine are the other two, and neither has "
            f"anything to do with the tests. Establish which before touching one.")
    return parse_survivors(out), parse_timed_out(out)


# gremlins derives each mutant's timeout from a BASELINE run of the package
# suite, and that baseline moves with machine state: in a full gate run, where
# mcp follows seven warm packages, the baseline is fast and the timeout tight;
# run alone after a code change, compilation inflates it and the timeout is
# generous. internal/mcp measured 58 timeouts, then 1, then 58 again across
# three runs of identical code — the measurement, not the code, was moving.
#
# 30 rather than 10 buys headroom so a slow-but-correct mutant is not mistaken
# for a hung one. It costs wall clock and nothing else, which per Patrik's
# ruling (2026-07-28) is not a reason to prefer the flakier number.
TIMEOUT_COEFFICIENT = "30"


PACKAGE_CLAUSE_RE = re.compile(r"^package\s+(\w+)")


def declared_package(pkg_dir):
    """The package name declared by a directory's non-test .go files."""
    try:
        names = sorted(os.listdir(pkg_dir))
    except OSError:
        return None
    for name in names:
        if not name.endswith(".go") or name.endswith("_test.go"):
            continue
        with open(os.path.join(pkg_dir, name)) as fh:
            for line in fh:
                m = PACKAGE_CLAUSE_RE.match(line)
                if m:
                    return m.group(1)
    return None


def unresolvable_packages(packages, root="."):
    """Gated packages gremlins cannot resolve a test target for.

    gremlins picks a mutant's test target by walking UP from the mutated file
    for a directory whose NAME equals the declared package name. A `package
    main` in a directory not named `main` has no such ancestor, so gremlins
    falls back to the bare MODULE PATH and runs `go test github.com/owner/repo`
    — which does not resolve, exits 1, and exit 1 is precisely what gremlins
    scores as KILLED. Every mutant becomes a false kill in ~11ms, and the gate
    prints a clean measurement for a run that never executed a single test.

    This is the gate holding itself to its own standard. It exists because the
    failure is INVISIBLE from the outside: the output is indistinguishable from
    a genuinely well-tested package, and sampling a killable mutant to check
    cannot detect it — a constant "everything dies" verdict passes that check
    too. Only a provably EQUIVALENT mutant reported as killed gives it away.

    Found 2026-08-04 by review, in a change adding cmd/vtt to PACKAGES on the
    strength of "91 killed, 0 lived, ~9s" — while `go test ./cmd/vtt/` alone
    takes 29s. tools/toolgen had been in the gated set with the same defect,
    never measured. tools/mutation-scope.md carries both, with the manual
    measurements taken in their place.

    Returns [(pkg, declared_name, directory_name), ...]; empty is good.
    """
    bad = []
    for pkg in packages:
        rel = _norm_pkg(pkg)
        declared = declared_package(os.path.join(root, rel))
        if declared is None:
            # No Go source to read. run_package will fail on its own terms;
            # inventing a second diagnosis here would only obscure that one.
            continue
        dirname = os.path.basename(rel)
        if declared != dirname:
            bad.append((pkg, declared, dirname))
    return bad


# Symlinks that exist in the tree and are accounted for. The value is the
# REASON, and a reason must name which packages the symlink makes unmeasurable
# — that is the entire point of recording it. A bare "we know about this one"
# would let the next person gate a package this entry already forbids.
ALLOWED_SYMLINKS = {
    # Empty, and that is the healthy state. The repo carried exactly one
    # tracked symlink — scenarios/testdata/dnd45e-minimal-adventures/
    # goblin-ambush — which existed only because `serve --adventures-dir
    # ./adventures` could not boot a mixed-ruleset library. loadAdventuresDir
    # now selects by ruleset, so the fixture is gone and cmd/vtt's tests no
    # longer depend on a path gremlins drops.
    #
    # An entry here must name which packages the symlink makes unmeasurable;
    # that is the whole point of recording one rather than merely tolerating
    # it. Prefer removing the symlink to adding an entry.
}

# Not part of the Go module, and node_modules is full of symlinks: walking it
# would make this guard both slow and permanently red.
UNWALKED_DIRS = {".git", "node_modules"}


def dropped_symlinks(root=".", allowed=None):
    """Symlinks in the tree that are not recorded in ALLOWED_SYMLINKS.

    gremlins copies the module before mutating it, and that copy DROPS
    symlinks: `copyPath` (workdir.go:142) switches on `mode.IsDir()` and
    `mode.IsRegular()`, a symlink is neither, so it falls through the switch
    and is skipped with no error. The path is simply absent from the copy.

    A test that reads it then fails in the copy for EVERY mutant, exiting 1 —
    and exit 1 is precisely what gremlins scores as KILLED
    (getTestFailedStatus, executor.go:257). So the package reports perfect
    efficacy having detected nothing. Same constant verdict as
    unresolvable_packages() above, reached by a different route, and just as
    invisible from the output.

    Only exit 1 does this. Exit 2 is NOT VIABLE and every OTHER non-zero exit
    is scored LIVED, so the neighbouring failure inverts: a suite killed by a
    signal yields false SURVIVORS rather than false kills. The distinction is
    worth keeping straight — the two mistakes have opposite symptoms.

    Found 2026-08-05: cmd/vtt's published "75 killed, no genuine survivors
    among the 77 evaluated" was this, not a well-tested package. Its scenario
    tests boot against scenarios/testdata/dnd45e-minimal-adventures, whose only
    entry is a symlink, and in the copy that directory is empty.

    Returns repo-relative paths, sorted; empty is good.
    """
    if allowed is None:
        allowed = ALLOWED_SYMLINKS
    for path, reason in sorted(allowed.items()):
        if not reason or not reason.strip():
            raise EquivalentsError(
                f"ALLOWED_SYMLINKS[{path!r}] has no reason. An excuse with no argument is how "
                f"a real gap becomes a permanent one in writing — the reason must name which "
                f"packages this symlink makes unmeasurable.")

    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        # Check BEFORE pruning. A symlink NAMED `node_modules` is a dropped
        # symlink like any other, and skipping it for its name would give the
        # guard a hole shaped exactly like its own exclusion list.
        for name in list(dirnames) + filenames:
            full = os.path.join(dirpath, name)
            if not os.path.islink(full):
                continue
            rel = os.path.relpath(full, root).replace(os.sep, "/")
            if rel not in allowed:
                found.append(rel)
        dirnames[:] = [d for d in dirnames if d not in UNWALKED_DIRS]
    return sorted(found)


def gremlins_args(pkg, packages=PACKAGES):
    """The gremlins invocation for one package, excluding its GATED children.

    `unleash ./internal/rules/` RECURSES into subdirectories and reports those
    mutants relative to the package it was given — `conformance/conformance.go`
    rather than `internal/rules/conformance/conformance.go`. With both the
    parent and the child in PACKAGES that is wrong twice: the same mutant is
    measured under two different keys, so an adjudication written for the child
    does not match the parent's report and an already-excused survivor is
    called unadjudicated; and it costs the runtime twice (internal/adventure's
    run carried 23 of its child's mutants, internal/rules' carried 60).

    Only GATED children are excluded. A subdirectory that is not separately in
    PACKAGES is measured here or nowhere, and dropping it would trade a visible
    gap for an invisible one — the failure this whole gate exists to prevent.

    Exclusion goes through gremlins' own --exclude-files rather than filtering
    its output, per this file's header: 8 of 9 defects across five review
    rounds of the coverage gate lived in hand-rolled enforcement code, and the
    fix was always to hand the job back to the toolchain.

    THE REGEXP IS ANCHORED TO THE PACKAGE-RELATIVE PATH, which is what makes a
    bare `^conformance/` correct and is the one thing here no unit test can
    own -- it is a property of the pinned gremlins, not of this code. In
    v0.6.0: engine.go:73 roots an os.DirFS at the directory unleash was given,
    engine.go:104 walks it from ".", and exclusion/rules.go:43 does a plain
    regexp MatchString against that relative path with no (?m), so `^` is
    start-of-text. Consequently `conformance_helpers.go` cannot match (needs a
    literal `/`) and `foo/conformance/bar.go` cannot either (not at position
    0). RE-VERIFY THOSE THREE LINES ON A VERSION BUMP; the tests below assert
    the argv, not gremlins' honouring of it.
    """
    args = ["go", "tool", "gremlins", "unleash", pkg,
            "--workers", "1", "--timeout-coefficient", TIMEOUT_COEFFICIENT,
            # WHICH VERDICTS GET PRINTED, pinned explicitly — l,c,t,k,v,s being
            # lived, not-covered, timed-out, killed, not-viable, skipped
            # (report/logger.go:58-74). 'r' (runnable) is omitted: it only
            # occurs under --dry-run.
            #
            # This closes a hole that had nothing to do with disks. The flag
            # defaults to EMPTY and gremlins binds it through viper, so
            # GREMLINS_UNLEASH_OUTPUT_STATUSES in the environment — or a
            # .gremlins.yaml anywhere up the tree — beats an unset flag. With
            # `=k` exported, MEASURED 2026-08-10 on internal/adventure, the
            # LIVED line vanishes while the summary still says `Lived: 1`. This
            # gate reads survivors from the LINES, so it would have reported
            # zero survivors and PASSED over a real one. An environment
            # variable could silence the gate.
            "--output-statuses", "lctkvs"]
    parent = _norm_pkg(pkg)
    for other in packages:
        child = _norm_pkg(other)
        # Trailing slash on the prefix so internal/rulesets is not read as a
        # child of internal/rules.
        if child != parent and child.startswith(parent + "/"):
            rel = child[len(parent) + 1:]
            args += ["--exclude-files", f"^{re.escape(rel)}/"]
    return args


def clear_test_cache_args():
    """The command that makes every mutant's deadline mean the same thing.

    -testcache, NOT -cache, and the difference is the whole cost of this fix:
    `go clean -testcache` discards cached test RESULTS and leaves every
    compiled artifact alone, so nothing is rebuilt. `go clean -cache` throws
    away the build cache and costs minutes of recompilation on every gate run.
    """
    return ["go", "clean", "-testcache"]


def clear_test_cache():
    """Discard cached test results, so gremlins' baseline is a real measurement.

    WHY THIS EXISTS, read from gremlins v0.6.0 and then measured. Every mutant's
    deadline is `cProfile.Elapsed * TIMEOUT_COEFFICIENT` (engine/executor.go:101),
    and Elapsed is the wall time of gremlins' OWN coverage run — which it invokes
    as `go test -cover -coverprofile <file> <pkg>` with NO -count=1
    (coverage/coverage.go:145-157). That is eligible for Go's test result cache.

    Measured on internal/gateway, back to back:

        run 1   9.286s          -> deadline ~336s   suite 9.3s   fine
        run 2   (cached) 0.27s  -> deadline ~8.1s   suite 9.3s   ALL time out

    A 41x collapse, landing the deadline BELOW the suite's own runtime. That is
    the 55-of-78 gateway timeouts, and the internal/mcp history recorded at
    TIMEOUT_COEFFICIENT (58 timeouts, then 1, then 58 over identical code) with
    no appeal to machine load at all.

    Failure here is NOT fatal. A gate that refuses to run because it could not
    clear a cache has turned a slow measurement into no measurement; the guard
    that catches a collapsed deadline is the majority-timeout check, which stays.
    """
    try:
        subprocess.run(clear_test_cache_args(), capture_output=True, check=False)
    except OSError:
        pass


# THE SKIP CACHE, AND EVERY PATH THROUGH IT FAILS TOWARDS RUNNING.
#
# A mutation run costs ~32 minutes across twelve packages, which is why the
# whole TASK gets skipped, and that is how ddf2d96 landed red and survived three
# commits on top of it (nine by 2026-08-28). Note what that does NOT claim: the
# check that caught ddf2d96 was suspect_positions, now entry_faults, which runs
# BEFORE gremlins and costs milliseconds, so the 32 minutes never delayed the
# finding — they deterred the run that would have produced it. The fix is not
# to run less carefully; it is to stop paying for packages whose verdict cannot
# have changed, so that running the gate stops being a decision. Nothing here may ever make the gate
# QUIETER: a skip prints a line naming what was reused, so a thinner summary is
# never a silent one.
#
# The cache is machine-local and gitignored (Patrik, 2026-08-28). A committed
# cache would make "this package passed" a claim in git that can be stale,
# wrong, or conflicted — the exact class of defect this repo spent 2026-08-27
# on. A fresh clone runs everything once, which is the right default.


def package_fingerprint(pkg, root=".", lister=None, go_version=None):
    """A hash of everything that can change this package's mutation verdict.

    None means "cannot tell", which the caller must read as "run it".

    THE CLOSURE IS `-test`, AND THAT IS NOT A DETAIL. gremlins kills a
    package's mutants by running `go test` on it, which compiles the TEST
    binary — so the tests' imports decide verdicts just as much as the
    package's own. Measured 2026-08-28: internal/identity's non-test closure
    omits contract/gen, internal/store AND internal/testdb, all three of which
    its tests compile. Fingerprinting the non-test closure would have left a
    change to internal/testdb unable to re-run internal/identity, and the gate
    would have gone quietly narrower for exactly the packages whose tests reach
    furthest. Scoping by directory rather than import path keeps the module
    filter to one prefix comparison and needs no module path lookup.

    THE TOOLCHAIN IS IN THE HASH TOO. A Go upgrade changes generated code and
    inlining, and TIMEOUT_COEFFICIENT decides which mutants time out rather
    than live — both move verdicts without touching a source byte.
    """
    root = os.path.abspath(root)
    try:
        if lister is None:
            proc = subprocess.run(
                ["go", "list", "-deps", "-test", "-f", "{{.Dir}}", pkg],
                capture_output=True, text=True, cwd=root, timeout=120)
            if proc.returncode != 0:
                return None
            dirs = proc.stdout.split()
        else:
            dirs = lister(pkg)
        if go_version is None:
            gv = subprocess.run(["go", "version"], capture_output=True, text=True, timeout=60)
            go_version = gv.stdout.strip() if gv.returncode == 0 else None
            if not go_version:
                return None

        digest = hashlib.sha256()
        digest.update(go_version.encode("utf-8"))
        # THE INVOCATION, not just the coefficient. gremlins_args derives
        # --exclude-files for a parent from PACKAGES, so removing a gated CHILD
        # changes what its parent measures while leaving every source byte
        # alone. Hashing the argv covers the coefficient, the exclusion set and
        # the status flags together, and cannot drift from them.
        digest.update("\x00".join(gremlins_args(pkg)).encode("utf-8"))
        # go.mod and go.sum, because gremlins itself is pinned there
        # (go.mod's `tool (` block) and third-party source is correctly outside
        # the module filter below. Without these a version bump leaves all
        # twelve fingerprints unchanged, every package skips, and the new
        # mutator never runs — while gremlins_args' own doc tells you to
        # RE-VERIFY on a bump, in a run that would never happen.
        for meta in ("go.mod", "go.sum"):
            mp = os.path.join(root, meta)
            if os.path.isfile(mp):
                with open(mp, "rb") as fh:
                    digest.update(fh.read())
        files = []
        for d in sorted(set(dirs)):
            ad = os.path.abspath(d)
            if not (ad == root or ad.startswith(root + os.sep)):
                continue  # outside the module: stdlib and third-party
            # EVERY FILE, NOT JUST *.go, AND RECURSIVELY. This read
            # `listdir` + endswith(".go") until 2026-08-28, and review proved
            # the hole: seven of the twelve gated packages have testdata,
            # fixtures or //go:embed assets in their -test closure — around a
            # thousand files — and none of them moved the fingerprint.
            # internal/mapdef drives every assertion off testdata/valid/
            # cellar.json; internal/rules/schema/*.json is EMBEDDED production
            # input. Weaken a boundary case in a fixture, mutants that were
            # killed now live, the fingerprint does not move, the package is
            # skipped, and the gate prints "zero unadjudicated survivors".
            # That is the gate lying, which is the one thing it may not do.
            #
            # THE PRUNE IS NOT DEFENSIVENESS. If a doc.go ever lands at the
            # module root then root itself joins `dirs`, and an unpruned walk
            # would hash reports/mutation/mutation.json — 238 MB, every run.
            for dirpath, dirnames, names in os.walk(ad):
                dirnames[:] = [n for n in dirnames
                               if not n.startswith(".") and n not in FINGERPRINT_SKIP_DIRS]
                for n in names:
                    files.append(os.path.join(dirpath, n))
        if not files:
            return None  # a closure with no source is a lookup that went wrong
        for f in sorted(files):
            digest.update(os.path.relpath(f, root).encode("utf-8"))
            with open(f, "rb") as fh:
                digest.update(fh.read())
        return digest.hexdigest()
    except (OSError, ValueError, subprocess.SubprocessError):
        return None


def load_skip_cache(path):
    """The recorded verdicts, or {} — an unreadable cache runs everything.

    EVERY failure mode returns {}: no path, missing file, unparseable JSON, a
    payload that is not a dict. None of them may raise and none may skip, since
    a cache that cannot be read is indistinguishable from one that says
    "nothing is known", and that is exactly the answer that runs the gate.
    """
    if not path or not os.path.isfile(path):
        return {}
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError):
        return {}
    return data if isinstance(data, dict) else {}


def save_skip_cache(path, data):
    """Best effort. A cache that cannot be written costs time, never accuracy."""
    if not path:
        return
    try:
        os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
        # Write-then-rename. A truncate-in-place torn by two concurrent
        # `task check` runs is already recoverable — load_skip_cache returns {}
        # and everything runs — but needlessly so.
        tmp = path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as fh:
            json.dump(data, fh, indent=1, sort_keys=True)
        os.replace(tmp, path)
    except OSError:
        pass


def reusable_result(cache, name, fingerprint):
    """(survivors, timed_out) if this package provably cannot have changed.

    None means RUN IT, and that is the answer to every uncertainty: no
    fingerprint (the closure could not be computed), no entry, a fingerprint
    that differs, or an entry whose shape is not what was written. A skip has
    to be earned by an exact match on a recorded PASS; nothing else will do.
    """
    if not fingerprint:
        return None
    entry = cache.get(name)
    if not isinstance(entry, dict) or entry.get("fingerprint") != fingerprint:
        return None
    try:
        survivors = [(str(a), str(b)) for a, b in entry["survivors"]]
        timed_out = [(str(a), str(b)) for a, b in entry["timed_out"]]
    except (KeyError, TypeError, ValueError):
        return None
    return survivors, timed_out


def default_runner(pkg):
    proc = subprocess.run(gremlins_args(pkg), capture_output=True, text=True)
    return proc.stdout + proc.stderr


def _disk_targets():
    """The directories a mutation run consumes: the tree copies, and the cache."""
    targets = [tempfile.gettempdir()]
    cache = go_cache()
    if cache:
        targets.append(cache)
    return targets


def _free(path):
    try:
        return shutil.disk_usage(path).free
    except OSError:
        # An unreadable path must not silently lower the floor to nothing.
        return float("inf")


def run(equivalents_path, packages=PACKAGES, runner=default_runner,
        out=sys.stdout, err=sys.stderr, root=".", free_bytes=None,
        cache_bytes=None, prepare=clear_test_cache,
        fingerprint=None, skip_cache_path=None):
    # BEFORE anything is spent. Discovering a full disk as a corrupt verdict is
    # the failure this whole file's error handling is about; discovering it as
    # a number, up front, costs nothing.
    #
    # $TMPDIR rather than the repo: that is where gremlins copies the tree per
    # mutant, and on this platform it is a different volume from the source in
    # principle even though it is not in practice.
    if free_bytes is None:
        # BOTH volumes, whichever is tighter. $TMPDIR takes the per-mutant tree
        # copies; GOCACHE takes the retained growth, and they are not always
        # the same filesystem — a systemd /tmp is a RAM-backed tmpfs, where
        # reading only $TMPDIR would refuse forever on a machine with hundreds
        # of free gigabytes AND `go clean -cache` could not move the number by
        # a byte. The inverse is worse: /tmp looks roomy while the cache volume
        # is full, and the pre-flight waves through the run this exists to stop.
        free_bytes = min(_free(d) for d in _disk_targets())
    if free_bytes < MIN_FREE_BYTES:
        print(f"check:mutation: {free_bytes / 1024**3:.1f} GiB free on "
              f"{tempfile.gettempdir()}, and this needs at least "
              f"{MIN_FREE_BYTES / 1024**3:.0f} GiB. gremlins copies the tree per mutant and "
              f"every mutant is a distinct binary, so a full gate run costs about 7.6 GiB and "
              f"leaves GOCACHE ~4 GiB larger — unbounded across runs, measured at 82 GiB. "
              f"Run `go clean -cache` and try again. REFUSING RATHER THAN RUNNING: out of space, "
              f"gremlins fails to apply or restore mutations, scores the result anyway, prints a "
              f"normal summary and exits 0 — a wrong answer that looks exactly like a right one.",
              file=err)
        return 1

    # PAST the floor, so this run is going ahead. The floor is a cliff; this is
    # the ramp, while there is still room to act.
    if free_bytes < CACHE_WARN_FREE_BYTES:
        if cache_bytes is None:
            cache_bytes = cache_size(go_cache())
        recover = ("" if cache_bytes is None else
                   f" `go clean -cache` would recover about "
                   f"{cache_bytes / 1024**3:.1f} GiB of that.")
        print(f"check:mutation: {free_bytes / 1024**3:.1f} GiB free, heading for the "
              f"{MIN_FREE_BYTES / 1024**3:.0f} GiB floor that refuses to run at all. "
              f"Running anyway — this is a warning, not a floor. Every mutant is a "
              f"distinct binary, so the build cache grows without bound across runs; "
              f"measured at 82 GiB on a 228 GiB volume, and once the volume actually "
              f"fills, Bash stops working, so the failure does not even present as a "
              f"gate failure.{recover}",
              file=err)

    # AFTER the floor, so a refused run does not also throw away the test cache
    # somebody else's next `go test` would have used — and BEFORE the first
    # package, because clearing it later leaves that package judged against a
    # collapsed deadline, which is the failure rather than a smaller version.
    prepare()

    # BEFORE anything is measured: a package gremlins cannot resolve reports
    # every mutant as killed, so letting the loop below run would print `ok`
    # lines that establish nothing.
    unresolvable = unresolvable_packages(packages, root=root)
    if unresolvable:
        for pkg, declared, dirname in unresolvable:
            print(f"check:mutation: {pkg} declares `package {declared}` in a directory named "
                  f"`{dirname}`. gremlins resolves a test target by matching the directory name "
                  f"to the package name, so it falls back to the bare module path, `go test` "
                  f"exits 1 for an unresolvable package, and EVERY mutant is scored KILLED in "
                  f"milliseconds. The measurement would be worthless. Remove it from PACKAGES "
                  f"and record it in tools/mutation-scope.md.", file=err)
        return 1

    # Also before anything is measured: a symlink gremlins drops silently makes
    # every test that reads it fail in the copy, under every mutant.
    try:
        stray = dropped_symlinks(root=root)
    except EquivalentsError as exc:
        print(f"check:mutation: {exc}", file=err)
        return 1
    if stray:
        for path in stray:
            print(f"check:mutation: {path} is a symlink. gremlins' workdir copy switches on "
                  f"IsDir()/IsRegular() and drops symlinks with no error, so any test that reads "
                  f"this path fails in the copy under EVERY mutant — and exit 1 is what gremlins "
                  f"scores as KILLED. Replace it with a real file or directory. If it must stay "
                  f"AND it is committed, record it in ALLOWED_SYMLINKS with the packages it makes "
                  f"unmeasurable — never record an untracked local one, which would red the gate "
                  f"for every other clone.", file=err)
        return 1

    try:
        entries = read_equivalents(equivalents_path)
    except (EquivalentsError, OSError) as exc:
        print(f"check:mutation: {exc}", file=err)
        return 1

    # BEFORE the run, because it costs milliseconds and the reader should have
    # it in front of them for the half hour the run takes. NOT a `return`: the
    # run still measures the code correctly, and the same invocation goes on to
    # print each unadjudicated survivor's code ready to compare with the entry.
    faults = entry_faults(entries, root=root)
    for e, fault in faults:
        print(f"check:mutation: {equivalents_path} lists {describe(e)}, but {fault}. AN "
              f"ADJUDICATION THE TREE REFUTES EXCUSES NOTHING, which is worse than an "
              f"unadjudicated survivor. Read the source, find the code this entry's reason "
              f"describes, and correct the entry — do not delete it.", file=err)

    unadjudicated = []
    unreadable = []
    stale = []
    seen = set()
    unevaluated = 0
    skip_cache = load_skip_cache(skip_cache_path)
    fresh_cache = {}
    for pkg in packages:
        name = _norm_pkg(pkg)
        fp = None
        if fingerprint is not None:
            fp = fingerprint(pkg)
        source = package_source(root, pkg)
        reused = reusable_result(skip_cache, name, fp)
        if reused is not None:
            survivors, timed_out = reused
            fresh_cache[name] = skip_cache[name]
            print(f"..  {name}: UNCHANGED since its last passing run, reusing that "
                  f"verdict ({len(survivors)} survivor(s), {len(timed_out)} timed out). "
                  f"Nothing in its dependency closure differs.", file=out)
        else:
            try:
                survivors, timed_out = run_package(pkg, runner)
            except EquivalentsError as exc:
                print(f"check:mutation: {exc}", file=err)
                return 1
        survivors = [tuple(s) for s in survivors]
        timed_out = [tuple(t) for t in timed_out]
        lived, cannot = [], []
        for location, mutator in survivors:
            key = resolve(source, name, location, mutator)
            if key is None:
                cannot.append((name, location, mutator))
            else:
                lived.append((key, (location, mutator)))
        # A timed-out mutant whose line cannot be read is no survivor, so it
        # fails nothing; it just cannot hold an entry.
        hung = [(key, t) for t in timed_out
                if (key := resolve(source, name, t[0], t[1])) is not None]
        mine = [e for e in entries if e.package == name]
        seen.add(name)
        claimed, unclaimed, gone = claims(mine, sorted(lived, key=_by_position),
                                          sorted(hung, key=_by_position))
        stale += gone
        unreadable += cannot
        for key, mutant in unclaimed:
            sharing = sorted(m[0] for k, m in lived if k == key)
            named = sum(1 for e in mine if entry_key(e) == key)
            unadjudicated.append((name, mutant, key, sharing, named,
                                  copies_in(source, key[1], key[5])))
        if fp is not None and not unclaimed and not cannot:
            fresh_cache[name] = {
                "fingerprint": fp,
                "survivors": [list(s) for s in survivors],
                "timed_out": [list(t) for t in timed_out],
            }
        # A TIMED OUT MUTANT CAN HOLD AN ENTRY. A timeout is a detection for
        # the verdict (Patrik, 2026-07-28) but NOT an evaluation, so it is no
        # evidence that an adjudication expired: an entry on its code is not
        # stale. Nothing is excused that would otherwise fail, since a timed-out
        # mutant never joins `unadjudicated` either.
        claimed_timeouts = [t for t in timed_out if t in claimed]

        adjudicated = len(survivors) - len(unclaimed) - len(cannot)
        note = f", {len(timed_out)} TIMED OUT (counted as killed, NOT measured)" if timed_out else ""
        if claimed_timeouts:
            note += (f"; {len(claimed_timeouts)} of those hold an adjudication this run "
                     f"did not re-test, so the entry count will exceed the adjudicated count")
        print(f"ok  {name}: {len(survivors)} survivor(s), {adjudicated} adjudicated{note}", file=out)
        # Name them. A timed-out mutant can be a genuine survivor (see
        # MAX_TIMEOUT_FRACTION), so the one thing the gate must not do is let
        # them vanish between gremlins' output and this report.
        for location, mutator in timed_out:
            print(f"    timed out: {name} {location} {mutator} — NOT EVALUATED, "
                  f"counted as killed", file=out)
        unevaluated += len(timed_out)

    # ONCE, not once per mutant, and naming all THREE causes this repo has
    # actually measured. The advice used to ride every timeout line and name
    # only the first — which on 2026-08-29 sent a session hunting a hang that
    # did not exist. See check_mutation_test.py's TimeoutAdviceTest.
    #
    # The mcp "58 timeouts, then 1, then 58 over identical code" history is
    # deliberately NOT cited here. TIMEOUT_COEFFICIENT attributes it to
    # compilation, clear_test_cache to the cached baseline "with no appeal to
    # machine load at all", and tools/mutation-scope.md calls the cache the
    # likeliest and this file's attribution wrong. A message is the wrong place
    # to settle that, and citing it under any one cause is how it got contested.
    if unevaluated:
        print(f"\ncheck:mutation: {unevaluated} mutant(s) above were NOT EVALUATED and are "
              f"counted as killed. THREE different failures wear that label here and the fix "
              f"for one is wrong for the others. (1) A TEST THAT BLOCKS under the mutant, "
              f"hanging the suite instead of failing it — make it fail fast. (2) A COLLAPSED "
              f"DEADLINE: every mutant's timeout is gremlins' own coverage-run time times "
              f"TIMEOUT_COEFFICIENT, and that coverage run carries no -count=1, so a warm Go "
              f"test cache shrinks it — measured 41x, to below the suite's own runtime "
              f"(clear_test_cache, this file, which is why the gate clears that cache first). "
              f"(3) A RUN STARVED of the machine, where nothing blocks and no deadline moved: "
              f"on 2026-08-29 all 179 mutants in one client file timed out inside a full `task "
              f"check` while a standalone run of that same file killed 172 of them. DO NOT "
              f"CHANGE A TEST on one run's count. The deadline moves with the baseline, so two "
              f"runs are comparable only at the same cache state and the same machine load — "
              f"tools/mutation-scope.md: compare like with like, and state the cache state.",
              file=out)

    failed = bool(faults) or bool(unreadable)
    stale += [e for e in entries if e.package not in seen]
    # An entry the tree refutes is stale as well, and gets the one message
    # above: "remove the entry" beside "do not delete it" would lose the reason.
    faulted = {id(e) for e, _ in faults}
    stale = [e for e in stale if id(e) not in faulted]

    for name, location, mutator in unreadable:
        print(f"check:mutation: {name} {location} {mutator} SURVIVED, and its line cannot be read "
              f"from the source the run measured, or its column falls outside the code on it, so "
              f"the gate cannot tell which code it sits on. It claims nothing and fails (VTT-304).",
              file=err)
        failed = True

    for name, (location, mutator), key, sharing, named, copies in unadjudicated:
        note = ""
        if len(sharing) > 1:
            note = (f" ({len(sharing)} survive on that code, at {', '.join(sharing)}, and "
                    f"{named} {'entry names' if named == 1 else 'entries name'} it)")
        print(f"check:mutation: {name} {location} {mutator} SURVIVED and is not adjudicated{note} — "
              f"write a test that kills it, or (only if no observable can distinguish it) add it "
              f"to {equivalents_path} with the reason:\n"
              f"{name}  {key[1]}  {mutator}  offset={key[3]}  nth={key[4]}  copies={copies}\n"
              f"    code: {key[5]}\n"
              f"    {PLACEHOLDER}", file=err)
        failed = True

    # A stale entry is one no surviving or timed-out mutant has the key of
    # (VTT-301). DO NOT have this line say why, or pick a remedy from the
    # survivors printed above it: the gate does not look up what became of
    # the mutant under the key, and a survivor beside the entry may be
    # nobody's, so the line names where the reader looks (VTT-308).
    for e in stale:
        print(f"check:mutation: {equivalents_path} lists {describe(e)}, which no longer survives "
              f"under that key (the fields and code just named): this run reports no surviving "
              f"or timed-out mutant under it, and that alone does not say the mutant was "
              f"killed. This run found this entry's code in that file on as many lines as the "
              f"entry records, so a survivor printed on other code is on another line than "
              f"that code, unless the file changed between that count and the measuring of its "
              f"mutants. Look above, in this same run's output, for a survivor in that file. "
              f"If one is printed unadjudicated, with an entry ready to paste under it, "
              f"compare its header and code line with this entry's: it may be this entry's "
              f"mutant under another key. Read this entry's reason against that mutant where "
              f"it stands in the file, not against the code line printed there alone. If the "
              f"reason describes that mutant and holds there, re-key this entry instead of "
              f"deleting it: give it the header and code line printed there. If it describes "
              f"that mutant and no longer holds, this entry is wrong: remove it. If it does "
              f"not describe that mutant, the survivor is another mutant. In both of those "
              f"cases that survivor needs a test or an entry of its own. If one is printed as "
              f"a survivor the gate cannot read (VTT-304), it may be this entry's mutant too: "
              f"leave this entry as it is until a run can read that survivor. When no survivor "
              f"above is this entry's mutant, make the mutation by hand and run the tests, "
              f"since this run's output does not show a kill: if a test now kills the mutant, "
              f"remove the entry so it cannot pre-approve a future survivor; if none does, "
              f"leave the entry and find out why this run did not report its mutant.",
              file=err)
        failed = True

    if failed:
        return 1
    # WRITTEN ONLY ON A CLEAN RUN, and only for packages that contributed no
    # unadjudicated survivor. A cache entry is a claim that this package passed;
    # recording one from a failing run would let the next run skip its way past
    # the failure, which is the one way this optimisation could become a
    # correctness bug rather than a speed one.
    save_skip_cache(skip_cache_path, fresh_cache)
    print(f"\ncheck:mutation: {len(packages)} packages, zero unadjudicated survivors.", file=out)
    return 0


# Where the skip cache lives. Under reports/, which .gitignore already covers,
# because it is a MACHINE-LOCAL fact and not a repo one: a committed cache would
# put "this package passed" into git where it can go stale, be wrong, or conflict
# on a merge. VTT_MUTATION_NO_SKIP=1 forces a full measurement — for when you
# want the number rather than the verdict, or do not trust the cache.
FINGERPRINT_SKIP_DIRS = {"node_modules", "reports", "webdist", ".task"}

SKIP_CACHE_PATH = os.path.join("reports", "mutation-skip-cache.json")


def main(argv):
    if len(argv) != 2:
        print("usage: check-mutation.py <equivalents-file>", file=sys.stderr)
        return 2
    if os.environ.get("VTT_MUTATION_NO_SKIP"):
        return run(argv[1])
    return run(argv[1], fingerprint=package_fingerprint,
               skip_cache_path=SKIP_CACHE_PATH)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
