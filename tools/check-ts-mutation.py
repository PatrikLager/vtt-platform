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
Stryker has no bun test-runner plugin, so it runs `bun test client/test` as an
opaque command and reads the exit code. The cost is no per-test filtering:
every mutant runs the whole suite. Measured, that is fine — the suite is
~700ms, and 1667 mutants complete in minutes. `coverageAnalysis: "off"` is
mandatory with the command runner, not a tuning choice.

This file parses JSON with the json module and compares two sets. It does not
implement mutation testing. That boundary is the same one check-mutation.py
draws, for the same reason recorded there: across five review rounds of the
coverage gate, 8 of 9 defects were in hand-rolled enforcement code.

IT DOES READ TYPESCRIPT SOURCE, in exactly one way, and this paragraph used to
say it did not. Each adjudication names a line:col, and the gate now reads that
one line to check the key still points at a mutant of the kind it names, and to
derive the anchor that pairs a stale entry to the survivor it became
(MUTATOR_TOKENS, read_position, pair_moves). That is a byte lookup and a prefix
comparison against Stryker's own replacement text — no parser, no AST, no
opinion about what the code means. But "does not interpret TypeScript" was
false the moment this landed, and a comment that was true when written and
stopped being so with nothing to notice is this repo's dominant defect.
"""

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


def read_equivalents(path):
    """{(file, "line:col", mutator, replacement): reason}.

    A header line plus an indented reason. The reason is REQUIRED. An
    equivalence claim asserts that NO test can ever kill the mutant, so it
    forecloses writing one; on the Go side two of four such claims were wrong,
    and a bare entry is how that happens quietly.
    """
    entries = {}
    pending = None
    for lineno, raw in enumerate(Path(path).read_text().splitlines(), 1):
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if raw[:1].isspace():
            if pending is None:
                raise EquivalentsError(f"{path}:{lineno}: reason with no entry above it")
            entries[pending] = line.strip()
            pending = None
            continue
        if pending is not None:
            raise EquivalentsError(f"{path}:{pending_line}: entry with no reason under it")
        # The REPLACEMENT is part of the key, and is the rest of the line
        # because it can contain spaces (`a.id <= b.id`).
        #
        # Without it, two mutants at one location with one mutator — Stryker
        # emits `-> true` AND `-> false` for the same ConditionalExpression —
        # collapse to a single key, and adjudicating either silently excuses
        # the other. Found by this file's own duplicate check on
        # client/src/player.ts:20:51, where the two are genuinely different
        # claims: forcing that branch to `true` returns what it would have
        # returned anyway, while forcing it to `false` does not.
        parts = line.split(maxsplit=3)
        if len(parts) != 4:
            raise EquivalentsError(
                f"{path}:{lineno}: want '<file> <line>:<col> <MUTATOR> <replacement>', "
                f"got {line.strip()!r}")
        key = (parts[0], parts[1], parts[2], parts[3])
        if key in entries:
            raise EquivalentsError(f"{path}:{lineno}: {key} listed twice")
        pending, pending_line = key, lineno
    if pending is not None:
        raise EquivalentsError(f"{path}:{pending_line}: entry with no reason under it")
    return entries


def mutants(report):
    """Yield (file, "line:col", mutator, status) for every mutant."""
    for path, entry in sorted(report.get("files", {}).items()):
        for m in entry.get("mutants", []):
            start = m.get("location", {}).get("start", {})
            where = f"{start.get('line', '?')}:{start.get('column', '?')}"
            # Replacement is normalised to one line: it is a key component, and
            # Stryker emits multi-line replacements for block mutants.
            replacement = " ".join(str(m.get("replacement", "")).split())
            yield path, where, m.get("mutatorName", "?"), replacement, m.get("status", "?")


# ---------------------------------------------------------------------------
# DOES THE KEY STILL POINT AT A MUTANT OF THE KIND IT NAMES?
#
# THE DANGEROUS CLASS OF DRIFT, and not the one the rest of this file worries
# about. A key that has merely MOVED fails the gate anyway, loudly: a stale
# entry beside an unadjudicated survivor. A key that has landed on a COMMENT
# fails NOTHING — Stryker generates no mutant there, so it never appears in the
# report, and the entry sits in the file silently pre-approving whatever DOES
# land on that line later. It is an excuse with no mutant behind it.
#
# Not hypothetical, and not rare: `wire.ts 292:13` and `316:7` sat on comment
# lines for THREE DAYS, and a canvas.ts key landed on one within an HOUR of
# being written, because a comment edit above it shifted the line. Nothing
# about editing a comment suggests a mutation key is downstream of it.
#
# THE COLUMN IS NOT THE OPERATOR, which is the one thing to know before reading
# the rule below. Stryker reports the START of the mutated node, so
# `undo.ts 39:26` points at the `e` of `e.sequence > best`, not at the `>`.
# The REPLACEMENT is what says where the operator is: it is the same expression
# with one operator rewritten, so the first place the source and the
# replacement diverge IS the operator, and comparing them pins the OPERANDS
# too. A key that drifted onto a different comparison is rejected even when a
# `>` sits in exactly the same place.
MUTATOR_TOKENS = {
    "EqualityOperator": ("===", "!==", "==", "!=", "<=", ">=", "<", ">"),
    "ArithmeticOperator": ("+", "-", "*", "/", "%"),
    "UnaryOperator": ("+", "-", "~"),
}

# A StringLiteral mutant replaces the whole literal, so there is no operator to
# find — but its column must OPEN one, in any of JavaScript's three quotes.
#
# WHAT THAT DOES NOT CATCH, measured rather than guessed, by moving every one
# of the 73 recorded keys a line and a column in each direction and re-running
# the check: every position-checkable key that lands off its mutant is caught
# EXCEPT a few StringLiteral ones — at most three in any one direction, and
# every escape in all four directions was a StringLiteral. They escape for one
# reason, which is that the replacement is a sentinel (`""`, `"Stryker was
# here!"`) carrying no information about which literal it was. A key on `case
# "attackRolled":` moved one line onto `case "abilityUsed":` still opens a
# string; a key on the opening quote of `""` moved one column lands on its
# closing quote. Both stay undetectable without something in the file format to
# compare against, and the design's reason for not adding one is that a stored
# field would itself drift. The COMMENT rule still covers the drift that has
# actually bitten here, twice.
STRING_MUTATORS = ("StringLiteral",)

# Mutators whose column is checked for rule 1 ONLY, each with the reason it
# cannot take rule 2. This list is not a convenience: an entry here is a
# NARROWER check, so it is written down per mutator rather than left as a
# silent default, and a mutator in neither table FAILS (see read_position).
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
# comment. THE THIRD ONE IS THE ONE THAT SHIPPED BROKEN: the hand-rolled
# version of this check tested `startswith("//")`, which neither `/**` nor a
# continuation matches, and it reported "0 suspect" TWICE over a key that had
# drifted three lines onto a `/**`. A checker that cannot fail is worse than no
# checker, which is why every rule here has a test built on a key broken on
# purpose rather than only on a good key that passes.
COMMENT_PREFIXES = (b"//", b"/*", b"*")

REPO = Path(__file__).resolve().parent.parent


def source_line(root, path, where):
    """The raw BYTES of the line a key names, or None if it cannot be read.

    Bytes, not text: the column is a byte offset, and decoding first would move
    it on any line containing a non-ASCII character.
    """
    try:
        wanted = int(where.split(":")[0])
    except (ValueError, IndexError):
        return None
    try:
        with open(Path(root) / path, "rb") as fh:
            for n, raw in enumerate(fh, 1):
                if n == wanted:
                    return raw.rstrip(b"\r\n")
    except OSError:
        return None
    return None


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

    BOTH OPERANDS ARE PINNED, NOT JUST THE LEFT ONE, and the right one is the
    half that matters most. The equivalents file records the worst of the six
    keys its 2026-08-21 sweep had to fix as one that "landed on the SIBLING
    comparison, right shape wrong operand — the worst of the six, because it
    looks plausible at the new location". That is exactly a key one line off
    where the left operand and the operator still agree: `env.sequence >
    this.seenSeq` against `env.sequence > this.lastSeq`. Comparing what follows
    the operator too is what tells those two apart. The source runs on to the
    end of the line while the replacement ends with the expression, so it is a
    prefix test rather than an equality.
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


def read_position(root, key):
    """(fault, anchor) for one key.

    fault is "" when the position can still host this mutator, and a sentence
    naming what is wrong when it cannot. anchor is the trimmed text of the line
    the key sits on — the content half of the pairing signature below.
    """
    path, where, mutator, replacement = key
    line = source_line(root, path, where)
    if line is None:
        return (f"there is no such line in the tree — {path} is missing, unreadable, or shorter "
                f"than that", "")
    anchor = line.strip().decode("utf-8", "replace")
    if line.strip().startswith(COMMENT_PREFIXES):
        return (f"that line is a COMMENT ({anchor!r}), and Stryker generates no mutant on one",
                anchor)
    try:
        col = int(where.split(":")[1])
    except (ValueError, IndexError):
        return (f"{where!r} is not a line:col", anchor)
    if mutator in NO_POSITION_RULE:
        return ("", anchor)
    if mutator in STRING_MUTATORS:
        opener = line[col - 1:col] if 1 <= col <= len(line) else b""
        if opener not in (b'"', b"'", b"`"):
            return (f"column {col} of {anchor!r} holds {opener.decode('utf-8', 'replace')!r}, "
                    f"which opens no string or template literal", anchor)
        return ("", anchor)
    tokens = MUTATOR_TOKENS.get(mutator)
    if tokens is None:
        # FAIL CLOSED. The shipped version of this check had no branch for one
        # entry's mutator and skipped it in silence, which is the other half of
        # how it said "0 suspect" over a broken key. A mutator this gate cannot
        # place is a gate that is not checking, and it must say so.
        return (f"{mutator} is in neither MUTATOR_TOKENS nor NO_POSITION_RULE, so this gate "
                f"cannot say what its column should hold and is not checking this key at all. "
                f"Add it to whichever it belongs in, with the reason", anchor)
    if col < 1 or col > len(line):
        return (f"column {col} is past the end of {anchor!r}", anchor)
    was = operator_at(line[col - 1:], replacement.encode(), tokens)
    if not was:
        return (f"column {col} of {anchor!r} does not begin the expression {replacement!r} "
                f"names — no {mutator} operator ({' '.join(tokens)}) stands where the "
                f"replacement puts one", anchor)
    return ("", anchor)


def suspect_positions(entries, root="."):
    """[(key, fault)] for every entry whose position cannot host its mutator."""
    bad = []
    for key in sorted(entries):
        fault, _ = read_position(root, key)
        if fault:
            bad.append((key, fault))
    return bad


# ---------------------------------------------------------------------------
# PAIRING A STALE ENTRY TO THE SURVIVOR IT BECAME
#
# An edit ABOVE an adjudicated mutant shifts its line, and the gate used to
# report that as two unrelated failures — a stale entry here, an unadjudicated
# survivor there — with nothing saying they were the same mutant. The obvious
# response to "no longer survives, remove it" is then to DELETE a reasoned
# adjudication and leave the survivor unexplained. Measured: four times in one
# day, across both gates.
#
# TWO PASSES, AND THE ORDER IS THE WHOLE DESIGN.
#
#   1. (file, mutator, replacement, ANCHOR), the anchor being the trimmed
#      source line the key sits on. This separates mutants that share a mutator
#      and a replacement but live on different statements — the case that
#      defeats StringLiteral, whose replacement is `"Stryker was here!"` for
#      EVERY string literal in a file. It rescues pairings the counts alone
#      refuse: two entries against three survivors gives up on all five, while
#      the anchor can carve out the one-to-one inside it.
#
#   2. (file, mutator, replacement) — what this gate has always paired on — for
#      whatever pass 1 left. The anchor REFINES and never gates, and that is
#      deliberate rather than lazy: an entry whose line has genuinely shifted
#      reads its anchor off somebody else's code, so requiring the anchor to
#      match would break move detection in exactly the case it exists for.
#
# COUNTS DECIDE BOTH PASSES. One entry and one survivor is a move. N of each
# with N > 1 is paired BY POSITION ORDER and said to be — two identical
# statements in one function (app.ts's two `history.replaceState` calls) cannot
# be told apart by any content, and their adjudications are interchangeable
# *because* they are identical, while the alternative this gate used to advise
# was deleting sound adjudications AND leaving real survivors unexplained.
# N against M is neither: both sides are reported the old way, unguessed.
def _by_position(key):
    try:
        line, col = key[1].split(":")
        return (int(line), int(col))
    except ValueError:
        return (0, 0)


def _pair(entries, survivors, sig_of):
    """{entry: (survivor, ordered)} for every group whose two sides balance."""
    groups = {}
    for key in entries:
        sig = sig_of(key)
        if sig is not None:
            groups.setdefault(sig, ([], []))[0].append(key)
    for key in survivors:
        sig = sig_of(key)
        if sig is not None and sig in groups:
            groups[sig][1].append(key)

    moved = {}
    for was_side, now_side in groups.values():
        if len(was_side) != len(now_side):
            continue
        for was, now in zip(sorted(was_side, key=_by_position),
                            sorted(now_side, key=_by_position)):
            moved[was] = (now, len(was_side) > 1)
    return moved


def pair_moves(stale, unadjudicated, root="."):
    """{stale key: (survivor key, ordered)} — the re-keys a reader should make."""
    def coarse(key):
        return (key[0], key[2], key[3])   # file, mutator, replacement

    def anchored(key):
        anchor = read_position(root, key)[1]
        # An anchor that could not be read is not an anchor. Grouping on the
        # empty string would put every unreadable key in one bucket and pair
        # them against each other on nothing at all.
        return coarse(key) + (anchor,) if anchor else None

    moved = {}
    for sig_of in (anchored, coarse):
        taken = {now for now, _ in moved.values()}
        moved.update(_pair([k for k in stale if k not in moved],
                           [m for m in unadjudicated if m not in taken], sig_of))
    return moved


def check(report, equivalents, out=sys.stdout, err=sys.stderr, root=REPO):
    # FIRST, because it does not depend on the report at all: an adjudication
    # whose key does not describe the code is not excusing the mutant it names,
    # whatever the run said. Reported and carried, not returned on — the run
    # still measured the code correctly, and the pairing below is what tells a
    # reader which survivor each broken key should have been pointing at.
    suspect = suspect_positions(equivalents, root=root)
    for key, fault in suspect:
        print(f"check:ts-mutation: {key[0]}:{key[1]} {key[2]} (-> {key[3]}) is adjudicated "
              f"equivalent, but {fault}. AN ADJUDICATION THAT DOES NOT POINT AT ITS MUTANT "
              f"EXCUSES NOTHING AND PRE-APPROVES WHATEVER LANDS THERE NEXT, which is worse than "
              f"an unadjudicated survivor. Read the source, find the expression this entry's "
              f"reason describes, and re-key it — do not delete it.", file=err)

    survived, timed_out, killed, no_coverage, not_viable = [], [], 0, [], 0
    for path, where, mutator, replacement, status in mutants(report):
        if status == KILLED:
            killed += 1
        elif status == SURVIVED:
            survived.append((path, where, mutator, replacement))
        elif status == TIMEOUT:
            timed_out.append((path, where, mutator, replacement))
        elif status == NO_COVERAGE:
            no_coverage.append((path, where, mutator, replacement))
        elif status in NOT_VIABLE:
            not_viable += 1
        else:
            print(f"check:ts-mutation: unknown status {status!r} at {path}:{where}. "
                  f"Refusing to guess whether that is a detection.", file=err)
            return 1

    fail = bool(suspect)

    # Not covered means NO test reaches the code at all. With the command
    # runner that should be impossible — every mutant runs the whole suite —
    # so its appearance means the run was not configured the way this gate
    # assumes, and the numbers below do not mean what they say.
    for path, where, mutator, _ in no_coverage:
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
    for path, where, mutator, _ in timed_out:
        print(f"    timed out: {path}:{where} {mutator} — not evaluated; if this "
              f"persists, make the test that blocks under it fail fast", file=out)

    fraction = len(timed_out) / measured
    if fraction > MAX_TIMEOUT_FRACTION:
        print(f"check:ts-mutation: {len(timed_out)} of {measured} mutants timed out "
              f"({fraction:.0%}). Counting those as kills would score a broken run as a "
              f"good one — fix what hangs before trusting this.", file=err)
        fail = True

    # PAIR BEFORE REPORTING EITHER SIDE — see pair_moves above for what the two
    # passes are and why the anchor refines rather than gates.
    live = set(survived)
    stale_keys = sorted(set(equivalents) - live)
    unadjudicated = [m for m in survived if m not in equivalents]

    moved = pair_moves(stale_keys, unadjudicated, root=root)
    for was, (now, ordered) in sorted(moved.items()):
        why = ("PAIRED BY POSITION ORDER: several entries and the same number of survivors share "
               "that file, mutator, replacement and source line, so nothing can say which became "
               "which — and it does not matter, because identical statements have "
               "interchangeable reasons. Object if you know better; the alternative is deleting "
               "sound adjudications and leaving real survivors unexplained."
               if ordered else
               "Same mutator and same mutation, so an edit shifted it.")
        print(f"check:ts-mutation: ADJUDICATION MOVED {was[0]} {was[2]} (-> {was[3]}): "
              f"{was[1]} -> {now[1]}. {why} RE-KEY the entry to the new line; do NOT delete it, "
              f"or its reasoning is lost and the survivor is left unexplained.", file=err)
        fail = True

    paired = {now for now, _ in moved.values()}
    unadjudicated = [m for m in unadjudicated if m not in paired]
    # `not in moved` because a pairing already told the reader where to re-key.
    # `not in suspect` because a key on a comment has no mutant, so it is STALE
    # as well as suspect, and the generic stale line would fire beside the
    # position one: "re-key it, do not delete it" under "Remove the entry".
    # Two messages that contradict each other are worse than one, and a reader
    # following the second loses the reasoning — the exact harm this change
    # exists to stop. The entry is still reported and the gate still fails.
    stale_keys = [k for k in stale_keys
                  if k not in moved and k not in {key for key, _ in suspect}]
    for path, where, mutator, replacement in unadjudicated:
        print(f"check:ts-mutation: SURVIVED {mutator} at {path}:{where} (-> {replacement}) — no "
              f"test distinguishes the mutated code. Kill it, or adjudicate it equivalent "
              f"with a stated observable.", file=err)
        fail = True

    # A stale adjudication excuses whatever later lands at that location.
    for key in stale_keys:
        print(f"check:ts-mutation: {key[0]}:{key[1]} {key[2]} (-> {key[3]}) is adjudicated equivalent "
              f"but no longer survives — it was killed, moved, or the line shifted. "
              f"Remove the entry rather than let it pre-approve a future mutant.", file=err)
        fail = True

    if fail:
        return 1
    print(f"check:ts-mutation: {measured} mutants, {killed} killed, "
          f"{len(timed_out)} timed out, {len(survived)} adjudicated equivalent, "
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
