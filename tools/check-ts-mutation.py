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
Stryker has no bun test-runner plugin, so it runs `bun test --bail
--only-failures client/test` as an opaque command and reads the exit code.
The cost is no per-test filtering: every mutant is handed the whole suite,
and `--bail` ends a run at its first failing test. `coverageAnalysis: "off"`
is mandatory with the command runner, not a tuning choice.

This file parses JSON with the json module and compares two sets. It does not
implement mutation testing. That boundary is the same one check-mutation.py
draws, for the same reason recorded there: across five review rounds of the
coverage gate, 8 of 9 defects were in hand-rolled enforcement code.

IT READS TYPESCRIPT SOURCE, as lines of text and nothing more. An adjudication
names the code its mutant starts on, not a line (SPEC-020), so the gate reads
the line Stryker names for each survivor from the report's own copy of the file
and compares the trimmed text and the offset in it with each entry's. Before
that it checks each entry against the tree: its code stands in its file as
often as the entry records, and its offset holds what its mutator rewrites, by
a prefix comparison against Stryker's replacement text. No parser, no AST, no
opinion about what the code means.
"""

import collections
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


# An adjudication names the code it excuses, never its line (SPEC-020):
#
#     <file>  <mutator>  offset=<n>  span=<n>  nth=<n>  copies=<n>  <replacement, to end of line>
#         code: <the line the mutant starts on, trimmed>
#         <reason>
#
# `offset` is Stryker's column, in UTF-16 code units, less the line's
# indentation, so a line that moves or is re-indented keeps its entry and a
# line that is edited loses it. `span` is how much code the mutant covers, in
# UTF-16 code units from Stryker's start to its end, leaving out each line's
# indentation and trailing blanks and counting a line break as one: a
# ConditionalExpression on a whole test and one on its left operand start at
# one column. `copies` is how many lines of the file hold that code, and `nth`
# which of them the entry names, counted from the top: an entry on one copy
# never excuses a mutant on another. The REPLACEMENT stays in the key: Stryker emits `-> true` AND
# `-> false` for one ConditionalExpression, and adjudicating the harmless one
# must not excuse the other.
Entry = collections.namedtuple("Entry", "file mutator offset span nth copies replacement code reason")

# The reason the gate prints under a ready entry. Refused as a reason: pasted
# unedited, it would excuse a mutant nobody judged.
PLACEHOLDER = "<the observable that makes it equivalent>"


def entry_key(e):
    return (e.file, e.mutator, e.replacement, e.offset, e.span, e.nth, e.code)


def describe(e):
    return (f"{e.file} {e.mutator} (-> {e.replacement}) offset={e.offset} span={e.span} "
            f"nth={e.nth} on `{e.code}`")


def read_equivalents(path):
    """[Entry], in file order.

    A line whose FIRST character is `#` is a comment; a `#` anywhere else is
    text, in code, replacement and reason alike. Under each entry come its code
    line and its reason, both required: an equivalence claim forecloses ever
    writing a test for its mutant, and a bare entry is how a wrong one lands
    unnoticed.
    """
    want = ("want '<file>  <mutator>  offset=<n>  span=<n>  nth=<n>  copies=<n>  <replacement>', "
            "each <n> at least 1")
    entries, head, code, head_line = [], None, None, 0
    # Split on line feeds only, as the gate splits source: splitlines() would
    # also break a code line at a form feed or U+2028.
    for lineno, line in enumerate(Path(path).read_text().split("\n"), 1):
        line = line[:-1] if line.endswith("\r") else line
        if not line.strip() or line.startswith("#"):
            continue
        if line[0].isspace():
            text = line.strip()
            if head is None:
                raise EquivalentsError(f"{path}:{lineno}: reason with no entry above it")
            if code is None:
                # Trimmed as the gate trims source (INDENT), so a printed entry
                # pasted back names the same code.
                text = line.lstrip(INDENT).rstrip(INDENT)
                if not text.startswith("code: "):
                    raise EquivalentsError(
                        f"{path}:{lineno}: entry with no code line under it — the first line "
                        f"under an entry is `code: <the line its mutant starts on, trimmed>`")
                code = text[len("code: "):]
                continue
            if text == PLACEHOLDER:
                raise EquivalentsError(
                    f"{path}:{lineno}: entry with the printed placeholder for its reason — state "
                    f"the observable that makes it equivalent, or write the test instead")
            entries.append(Entry(*head, code, text))
            head, code = None, None
            continue
        if head is not None:
            raise EquivalentsError(f"{path}:{head_line}: entry with no "
                                   f"{'code line' if code is None else 'reason'} under it")
        parts = line.split(maxsplit=6)
        try:
            names = [x.split("=")[0] for x in parts[2:6]]
            if len(parts) != 7 or names != ["offset", "span", "nth", "copies"]:
                raise ValueError
            offset, span, nth, copies = (int(x.split("=", 1)[1]) for x in parts[2:6])
            if min(offset, span, nth, copies) < 1:
                raise ValueError
        except ValueError:
            raise EquivalentsError(f"{path}:{lineno}: {want}, got {line.strip()!r}") from None
        # Normalised as mutants() normalises Stryker's, so the two compare.
        head = (parts[0], parts[1], offset, span, nth, copies, " ".join(parts[6].split()))
        head_line = lineno
    if head is not None:
        raise EquivalentsError(f"{path}:{head_line}: entry with no "
                               f"{'code line' if code is None else 'reason'} under it")
    return entries


def mutants(report):
    """Yield (file, "line:col", mutator, replacement, status, end) for every
    mutant, `end` being Stryker's end as "line:col", or None."""
    for path, entry in sorted(report.get("files", {}).items()):
        for m in entry.get("mutants", []):
            start = m.get("location", {}).get("start", {})
            where = f"{start.get('line', '?')}:{start.get('column', '?')}"
            stop = m.get("location", {}).get("end")
            end = f"{stop.get('line', '?')}:{stop.get('column', '?')}" if isinstance(stop, dict) else None
            # Replacement is normalised to one line: it is a key component, and
            # Stryker emits multi-line replacements for block mutants.
            replacement = " ".join(str(m.get("replacement", "")).split())
            yield path, where, m.get("mutatorName", "?"), replacement, m.get("status", "?"), end


# ---------------------------------------------------------------------------
# WHAT AN ENTRY'S OFFSET MUST HOLD, checked against the tree before the report
# is read. An entry on a comment, or on code its mutator cannot rewrite, names
# no mutant Stryker makes, so it could only ever go stale.
#
# THE OFFSET IS NOT THE OPERATOR. Stryker reports the START of the mutated
# node, so an EqualityOperator mutant on `e.sequence > best` starts at the `e`.
# The REPLACEMENT says where the operator is: it is the same expression with one
# operator rewritten, so the first place the code and the replacement diverge
# IS the operator, and comparing them compares the operands too (operator_at).
MUTATOR_TOKENS = {
    "EqualityOperator": ("===", "!==", "==", "!=", "<=", ">=", "<", ">"),
    "ArithmeticOperator": ("+", "-", "*", "/", "%"),
    "UnaryOperator": ("+", "-", "~"),
}

# A StringLiteral mutant replaces the whole literal, so there is no operator to
# find — but its offset must OPEN one, in any of JavaScript's three quotes.
STRING_MUTATORS = ("StringLiteral",)

# Mutators whose offset is checked for the comment rule ONLY, each with the
# reason it cannot take the column rule. An entry here is a NARROWER check, so
# it is written down per mutator; a mutator in neither table FAILS.
#
#   ConditionalExpression  spans a whole clause — `case "x":`, an `if` test, a
#                          ternary arm — and its replacement is `true`, `false`
#                          or the clause itself. No token to look for.
#   BlockStatement         replaced by an empty block; the column is a `{`.
#   ArrayDeclaration       replacement is the sentinel `["Stryker was here"]`,
#                          and the column opens the array or the expression
#                          defaulting to it.
#   ObjectLiteral          same shape as ArrayDeclaration.
#   BooleanLiteral         the column holds `true`, `false` or the operand of a
#                          `!`, and the replacement is the negation — nothing a
#                          token table adds anything to.
#   OptionalChaining       the column is the start of the whole member access,
#                          and the removed `?.` is somewhere inside it.
#   MethodExpression       the replacement is the receiver with a call dropped;
#                          the column starts the receiver.
NO_POSITION_RULE = ("ConditionalExpression", "BlockStatement", "ArrayDeclaration",
                    "ObjectLiteral", "BooleanLiteral", "OptionalChaining", "MethodExpression")

# `//`, `/*`, and a line whose first non-space is `*` — the middle of a block
# comment. Test the third: a check of `//` alone misses `/**` and its
# continuation lines.
COMMENT_PREFIXES = (b"//", b"/*", b"*")

REPO = Path(__file__).resolve().parent.parent


def byte_offset(line, col):
    """Stryker's 1-based column -> a 0-based BYTE offset into `line`.

    THESE ARE NOT THE SAME NUMBER, and this function exists because the code
    below used to assume they were. Stryker's columns come from Babel, which
    indexes JavaScript strings — UTF-16 code units. `—` is ONE unit and THREE
    UTF-8 bytes, so on any line holding one, a column used directly as a byte
    offset lands two early and the operator is not where the check looks.

    Round-tripping through UTF-16-LE is exact rather than close. Counting
    decoded CHARACTERS would agree with Babel across the Basic Multilingual
    Plane and disagree on emoji, which JavaScript counts as two units and
    Python as one — a difference no test would notice until source containing
    one reached a gated file.

    The bug this fixes failed SAFE: a sound key was rejected, never a broken
    one accepted. It still had to go. A gate that refuses correct work over an
    em dash is one people learn to argue with, and this repo's comments are
    full of em dashes.
    """
    text = line.decode("utf-8", "replace")
    units = text.encode("utf-16-le")
    prefix = units[: max(col - 1, 0) * 2].decode("utf-16-le", "replace")
    return len(prefix.encode("utf-8"))


def _leading_token(text, tokens):
    """The longest of `tokens` that `text` starts with, or "".

    Longest-first so `>=` is never read as a bare `>`: the two are different
    mutations, and this is the text a reader is told the key sits on.
    """
    for tok in sorted(tokens, key=len, reverse=True):
        if text.startswith(tok.encode()):
            return tok
    return ""


def operator_at(tail, replacement, tokens):
    """The operator this replacement rewrote, as it stands in the source, or "".

    `tail` is the source from the key's column; `replacement` is Stryker's
    mutated text for the same node. They are the same expression apart from one
    operator, so walking forward to the first difference lands ON or JUST AFTER
    that operator — just after when the two share a first character, which
    `>` against `>=` does. Walking back from there to the first offset where
    BOTH sides start with an operator of this mutator's kind finds it, and
    everything before that offset is equal by construction.

    THE RIGHT OPERAND IS CHECKED TOO, NOT JUST THE LEFT ONE, and that is the
    half that matters most. The equivalents file records the worst of the six
    keys its 2026-08-21 sweep had to fix as one that "landed on the SIBLING
    comparison, right shape wrong operand — the worst of the six, because it
    looks plausible at the new location". That is exactly a key one line off
    where the left operand and the operator still agree: `env.sequence >
    this.seenSeq` against `env.sequence > this.lastSeq`. Comparing what follows
    the operator too is what tells those two apart.

    IT IS A PREFIX TEST, NOT AN EQUALITY, and that is a real gap rather than a
    detail: the source runs on to the end of the line while the replacement
    ends with the expression, so there is nothing to compare the end against.
    A recorded right operand that is a proper prefix of the one now in the
    source passes — `this.lastSeq` is accepted against a drifted
    `this.lastSeqExtra` — because no token boundary is required after it. The
    sibling case above is still caught; a longer namesake is not.
    """
    n = min(len(tail), len(replacement))
    i = 0
    while i < n and tail[i] == replacement[i]:
        i += 1
    for j in range(i, -1, -1):
        was = _leading_token(tail[j:], tokens)
        now = _leading_token(replacement[j:], tokens)
        if was and now and tail[j + len(was):].startswith(replacement[j + len(now):]):
            return was
    return ""


INDENT = " \t\r"


def _units(text):
    return len(text.encode("utf-16-le")) // 2


def _cut(text, start, stop):
    """`text` from UTF-16 unit `start` to `stop`, as Stryker's columns count."""
    return text.encode("utf-16-le")[2 * start:2 * stop].decode("utf-16-le", "replace")


def resolve(sources, path, where, mutator, replacement, end):
    """The key of the mutant Stryker reports from `where` to `end` ("line:col")
    in `path`, read from the report's own copy of that file, or None when its
    line cannot be read, its column falls outside the code on it, or its end
    is missing or outside the source.

    The report's source, not the tree: Stryker's positions name what it
    mutated, and the tree can have moved since (VTT-303).
    """
    text = sources.get(path)
    if not isinstance(text, str) or end is None:
        return None
    try:
        line, col = (int(x) for x in where.split(":"))
        eline, ecol = (int(x) for x in end.split(":"))
    except ValueError:
        return None
    lines = text.split("\n")
    if not 1 <= line <= eline <= len(lines) or not 1 <= ecol <= _units(lines[eline - 1]) + 1:
        return None
    raw = lines[line - 1]
    code = raw.strip(INDENT)
    offset = col - (len(raw) - len(raw.lstrip(INDENT)))
    if not code or not 1 <= offset <= _units(code):
        return None
    if eline == line:
        span = ecol - col
    else:
        # Each line's indentation and trailing blanks left out, each break one
        # unit, so re-indenting a block keeps its multi-line mutants' keys.
        covered = ([_cut(raw, col - 1, _units(raw)).rstrip(INDENT)]
                   + [x.strip(INDENT) for x in lines[line:eline - 1]]
                   + [_cut(lines[eline - 1], 0, ecol - 1).lstrip(INDENT)])
        span = _units("\n".join(covered))
    if span < 1:
        return None
    nth = sum(1 for x in lines[:line] if x.strip(INDENT) == code)
    return (path, mutator, replacement, offset, span, nth, code)


def copies_in(text, code):
    """How many lines of `text` hold `code`."""
    return sum(1 for raw in (text or "").split("\n") if raw.strip(INDENT) == code)


def entry_faults(entries, root="."):
    """[(Entry, fault)] for every entry the tree refutes, before the report is read.

    Refused: a file that cannot be read; code that stands in its file a number
    of times other than `copies` (VTT-302); an `nth` above `copies`; two
    entries for one key; code that is a comment; a column rule its mutator
    fails; a mutator in neither table (fail closed). `span` cannot be checked
    without a parser: a wrong one matches nothing, and fails as stale.
    """
    sharing = collections.Counter(entry_key(e) for e in entries)
    files = {}
    bad = []
    for e in entries:
        if e.file not in files:
            try:
                text = (Path(root) / e.file).read_text(encoding="utf-8")
                files[e.file] = [raw.strip(INDENT) for raw in text.split("\n")]
            except (OSError, UnicodeDecodeError):
                files[e.file] = None
        fault = _entry_fault(e, files[e.file], sharing[entry_key(e)])
        if fault:
            bad.append((e, fault))
    return bad


def _entry_fault(e, lines, sharing):
    if lines is None:
        return f"there is no such file in the tree — {e.file} is missing or unreadable"
    n = lines.count(e.code)
    if n != e.copies:
        return f"`{e.code}` stands {n} time(s) in {e.file}, and the entry records copies={e.copies}"
    if e.nth > e.copies:
        return f"it names copy nth={e.nth} of code that stands {e.copies} time(s) in {e.file}"
    if sharing > 1:
        return (f"{sharing} entries name this code, copy, mutator, replacement, offset and span, "
                f"and one mutant can be excused only once")
    code = e.code.encode("utf-8")
    if code.startswith(COMMENT_PREFIXES):
        return "its code is a COMMENT, and Stryker generates no mutant on one"
    if e.mutator in NO_POSITION_RULE:
        return ""
    off = byte_offset(code, e.offset)
    if e.mutator in STRING_MUTATORS:
        opener = code[off:off + 1] if off < len(code) else b""
        if opener not in (b'"', b"'", b"`"):
            return (f"offset {e.offset} of {e.code!r} holds "
                    f"{opener.decode('utf-8', 'replace')!r}, which opens no string or template "
                    f"literal")
        return ""
    tokens = MUTATOR_TOKENS.get(e.mutator)
    if tokens is None:
        return (f"{e.mutator} is in neither MUTATOR_TOKENS nor NO_POSITION_RULE, so this check "
                f"cannot say what its offset should hold and is not checking this entry at all. "
                f"Add it to whichever it belongs in, with the reason")
    if off >= len(code):
        return f"offset {e.offset} is past the end of {e.code!r}"
    if not operator_at(code[off:], e.replacement.encode(), tokens):
        return (f"offset {e.offset} of {e.code!r} does not begin the expression "
                f"{e.replacement!r} names — no {e.mutator} operator ({' '.join(tokens)}) stands "
                f"where the replacement puts one")
    return ""


def claims(entries, survivors, timed_out):
    """(claimed, unadjudicated, stale): entries matched to live mutants by key
    (SPEC-020).

    `survivors` and `timed_out` are [(key, mutant)]. Per key, survivors are
    claimed first and one per entry (VTT-300), in the order given; entries left
    over claim timed-out mutants, since a timeout is a detection and not an
    evaluation; entries left after that are stale (VTT-301). `unadjudicated` is
    [(key, mutant)].
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
    """Sort key of a (key, (path, "line:col", ...)) pair: file, line, column."""
    path, where = item[1][0], item[1][1]
    line, col = where.split(":")
    return (path, int(line), int(col))


def check(report, entries, out=sys.stdout, err=sys.stderr, root=REPO):
    # FIRST, because it does not depend on the report at all: an adjudication
    # the tree refutes excuses nothing, whatever the run said. Reported and
    # carried, not returned on — the run still measured the code correctly.
    faults = entry_faults(entries, root=root)
    for e, fault in faults:
        print(f"check:ts-mutation: {describe(e)} is adjudicated equivalent, but {fault}. AN "
              f"ADJUDICATION THE TREE REFUTES EXCUSES NOTHING, which is worse than an "
              f"unadjudicated survivor. Read the source, find the code this entry's reason "
              f"describes, and correct the entry — do not delete it.", file=err)

    survived, timed_out, killed, no_coverage, not_viable = [], [], 0, [], 0
    for path, where, mutator, replacement, status, end in mutants(report):
        if status == KILLED:
            killed += 1
        elif status == SURVIVED:
            survived.append((path, where, mutator, replacement, end))
        elif status == TIMEOUT:
            timed_out.append((path, where, mutator, replacement, end))
        elif status == NO_COVERAGE:
            no_coverage.append((path, where, mutator, replacement, end))
        elif status in NOT_VIABLE:
            not_viable += 1
        else:
            print(f"check:ts-mutation: unknown status {status!r} at {path}:{where}. "
                  f"Refusing to guess whether that is a detection.", file=err)
            return 1

    fail = bool(faults)

    # Not covered means NO test reaches the code at all. With the command
    # runner that should be impossible — every mutant is handed the whole
    # suite, with no per-test selection — so its appearance means the run was
    # not configured the way this gate assumes, and the numbers below do not
    # mean what they say.
    for path, where, mutator, *_ in no_coverage:
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
    for path, where, mutator, *_ in timed_out:
        print(f"    timed out: {path}:{where} {mutator} — NOT EVALUATED, counted as killed",
              file=out)

    # ONCE, not once per mutant — 322 copies of it in the 2026-08-28 run — and
    # naming the cause that was actually happening. See TimeoutAdvice in
    # check_ts_mutation_test.py.
    if timed_out:
        print(f"\ncheck:ts-mutation: {len(timed_out)} mutant(s) above were NOT EVALUATED and "
              f"are counted as killed. TWO different failures wear that label and the fix for "
              f"one is wrong for the other: A TEST THAT BLOCKS under the mutant, hanging the "
              f"suite instead of failing it — make it fail fast — or A RUN STARVED of the "
              f"machine, where nothing blocks at all. Measured 2026-08-29: all 179 mutants of "
              f"client/src/view/scene-plan.ts timed out inside a full `task check`, while a "
              f"standalone container run of that same file killed 172 of them in 298s and two "
              f"of the supposedly hung mutants died in about a second. THE COUNT IS A "
              f"MEASUREMENT OF THE MACHINE, not of the tests: over byte-identical inputs it "
              f"has read 31 (2026-08-25), 94 (08-27), 322 (08-28) and 31 again (08-29). "
              f"RE-RUN ON AN IDLE MACHINE before changing any test.", file=out)

    fraction = len(timed_out) / measured
    if fraction > MAX_TIMEOUT_FRACTION:
        print(f"check:ts-mutation: {len(timed_out)} of {measured} mutants timed out "
              f"({fraction:.0%}). Counting those as kills would score a broken run as a "
              f"good one. Either a test BLOCKS under the mutant or the machine was loaded "
              f"and nothing blocked at all; they are indistinguishable from here, so re-run "
              f"on an idle machine before changing a test.", file=err)
        fail = True

    # A TIMED OUT MUTANT CAN HOLD AN ENTRY. A timeout is a detection for the
    # verdict (Patrik, 2026-07-28) but NOT an evaluation, so it is no evidence
    # that an adjudication expired: an entry on its code is not stale. Nothing
    # is excused that would otherwise fail, since a timed-out mutant never
    # becomes unadjudicated either.
    sources = {path: entry.get("source") for path, entry in report.get("files", {}).items()}
    lived, unreadable = [], []
    for m in survived:
        key = resolve(sources, *m)
        if key is None:
            unreadable.append(m)
        else:
            lived.append((key, m))
    hung = [(key, m) for m in timed_out if (key := resolve(sources, *m)) is not None]
    _, unclaimed, stale = claims(entries, sorted(lived, key=_by_position),
                                 sorted(hung, key=_by_position))
    # An entry the tree refutes is stale as well, and gets the one message
    # above: "remove the entry" beside "do not delete it" would lose the reason.
    faulted = {id(e) for e, _ in faults}
    stale = [e for e in stale if id(e) not in faulted]

    for path, where, mutator, replacement, _ in unreadable:
        print(f"check:ts-mutation: SURVIVED {mutator} at {path}:{where} (-> {replacement}), and "
              f"its line cannot be read from the report's own source, or its column falls "
              f"outside the code on it, or its end is missing or outside the source, so the "
              f"gate cannot tell which code it covers. It claims nothing and fails (VTT-304).",
              file=err)
        fail = True

    for key, (path, where, mutator, replacement, _) in unclaimed:
        sharing = sorted(f"{m[0]}:{m[1]}" for k, m in lived if k == key)
        named = sum(1 for e in entries if entry_key(e) == key)
        note = ""
        if len(sharing) > 1:
            note = (f" ({len(sharing)} survive on that code, at {', '.join(sharing)}, and "
                    f"{named} {'entry names' if named == 1 else 'entries name'} it)")
        print(f"check:ts-mutation: SURVIVED {mutator} at {path}:{where} (-> {replacement}){note} "
              f"— no test distinguishes the mutated code. Kill it, or adjudicate it equivalent "
              f"with a stated observable:\n"
              f"{path}  {mutator}  offset={key[3]}  span={key[4]}  nth={key[5]}  "
              f"copies={copies_in(sources.get(path), key[6])}  {replacement}\n"
              f"    code: {key[6]}\n"
              f"    {PLACEHOLDER}", file=err)
        fail = True

    # A stale entry names code no surviving or timed-out mutant sits on: the
    # mutant was killed, or the code was edited.
    for e in stale:
        print(f"check:ts-mutation: {describe(e)} is adjudicated equivalent but no longer survives "
              f"— no surviving or timed-out mutant sits on that code. If the code was edited, "
              f"re-judge the reason against the new code and re-key the entry; if a test now "
              f"kills the mutant, remove the entry so it cannot pre-approve a future one.",
              file=err)
        fail = True

    if fail:
        return 1
    print(f"check:ts-mutation: {measured} mutants, {killed} killed, "
          f"{len(timed_out)} timed out, {len(survived)} survivors all adjudicated equivalent, "
          f"zero unadjudicated survivors.", file=out)
    return 0


def main(argv):
    root = REPO
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
    # root EXPLICITLY, not by default: every key's position is resolved against
    # it, so which tree this reads is a fact of the invocation rather than of
    # whatever directory the gate happened to be started from.
    return check(json.loads(report_path.read_text()), equivalents, root=root)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: check-ts-mutation.py <equivalents-file>", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv))
